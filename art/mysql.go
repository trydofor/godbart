package art

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"strings"
	"time"
)

type MyConn struct {
	Pref *Preference
	Conn *sql.DB
	Name string
}

func (m *MyConn) Open(p *Preference, d *DataSource) (err error) {
	if p.DatabaseType != "mysql" {
		return errors.New("unsupported DatabaseType, need mysql, but " + p.DatabaseType)
	}

	db, err := sql.Open("mysql", d.Conn)
	if err != nil {
		return
	}

	db.SetMaxOpenConns(p.ConnMaxOpen)
	db.SetMaxIdleConns(p.ConnMaxIdel)

	rs, err := db.Query(`SELECT DATABASE()`)
	if err != nil {
		return
	}
	defer rs.Close()

	var n string
	if rs.Next() {
		err = rs.Scan(&n)
	}

	m.Pref = p
	m.Conn = db
	m.Name = n

	return
}

func (m *MyConn) DbConn() (db *sql.DB) {
	return m.Conn
}
func (m *MyConn) DbName() string {
	return m.Name
}

func (m *MyConn) Exec(qr string, args ...interface{}) (int64, error) {
	if rs, err := m.Conn.Exec(qr, args...); err != nil {
		return 0, err
	} else {
		return rs.RowsAffected()
	}
}

func (m *MyConn) Query(fn func(*sql.Rows) error, qr string, args ...interface{}) error {
	if rs, er := m.Conn.Query(qr, args...); er != nil {
		return er
	} else {
		defer rs.Close()
		return fn(rs)
	}
}

func (m *MyConn) Tables() (tbs []string, err error) {
	fn := func(rs *sql.Rows) (er error) {
		for rs.Next() {
			var val string
			if er = rs.Scan(&val); er != nil {
				return
			}
			tbs = append(tbs, val)
		}
		return
	}

	err = m.Query(fn, `SHOW TABLES`)
	return
}

func (m *MyConn) Columns(table string) (map[string]Col, error) {
	cls := make(map[string]Col)
	fn := func(rs *sql.Rows) (er error) {
		for rs.Next() {
			cl, nl := Col{}, ""
			if er = rs.Scan(&cl.Name, &cl.Seq, &cl.Deft, &nl, &cl.Type, &cl.Key, &cl.Cmnt, &cl.Extr); er != nil {
				return
			}
			cl.Null = strings.EqualFold(nl, "YES")
			cls[cl.Name] = cl
		}
		return
	}

	err := m.Query(fn, `
SELECT 
    COLUMN_NAME,
    ORDINAL_POSITION,
    COLUMN_DEFAULT,
    IS_NULLABLE,
    COLUMN_TYPE,
    COLUMN_KEY,
    COLUMN_COMMENT,
    EXTRA
FROM
    INFORMATION_SCHEMA.COLUMNS
WHERE
	TABLE_SCHEMA = ?
    AND TABLE_NAME = ?
`, m.Name, table)
	return cls, err
}

func (m *MyConn) Indexes(table string) (map[string]Idx, error) {
	ixs := make(map[string]Idx)
	fn := func(rs *sql.Rows) (er error) {
		for rs.Next() {
			ix, nq := Idx{}, 0
			if er = rs.Scan(&ix.Name, &nq, &ix.Cols, &ix.Type); er != nil {
				return
			}
			ix.Uniq = nq == 0
			ixs[ix.Name] = ix
		}
		return
	}

	err := m.Query(fn, `
SELECT 
    INDEX_NAME,
    GROUP_CONCAT(DISTINCT NON_UNIQUE) AS UNIQ,
    GROUP_CONCAT(DISTINCT COLUMN_NAME ORDER BY SEQ_IN_INDEX) AS COLUMN_NAME,
    GROUP_CONCAT(DISTINCT INDEX_TYPE) AS INDEX_TYPE
FROM
    INFORMATION_SCHEMA.STATISTICS
WHERE
    TABLE_SCHEMA = ?
    AND TABLE_NAME = ?
    GROUP BY INDEX_NAME;
`, m.Name, table)
	return ixs, err
}

func (m *MyConn) Triggers(table string) (map[string]Trg, error) {
	tgs := make(map[string]Trg)
	fn := func(rs *sql.Rows) (er error) {
		for rs.Next() {
			tg := Trg{}
			if er = rs.Scan(&tg.Name, &tg.Timing, &tg.Event, &tg.Statement); er != nil {
				return
			}
			tgs[tg.Name] = tg
		}
		return
	}

	err := m.Query(fn, `
SELECT 
    TRIGGER_NAME,
    ACTION_TIMING,
    EVENT_MANIPULATION,
    ACTION_STATEMENT
FROM 
    INFORMATION_SCHEMA.TRIGGERS 
WHERE 
    EVENT_OBJECT_SCHEMA=?
    AND EVENT_OBJECT_TABLE=?
`, m.Name, table)
	return tgs, err
}

func (m *MyConn) DdlTable(table string) (ddl string, err error) {
	fn := func(rs *sql.Rows) (er error) {
		var nm string
		if rs.Next() {
			if er = rs.Scan(&nm, &ddl); er != nil {
				return
			}
		}
		return
	}

	err = m.Query(fn, `SHOW CREATE TABLE `+table)
	return
}

func (m *MyConn) DdlTrigger(trigger string) (ddl string, err error) {
	fn := func(rs *sql.Rows) (er error) {
		cnt := 7
		var col = make([]string, cnt)
		var ptr = make([]interface{}, cnt)
		for i := range col {
			ptr[i] = &col[i]
		}
		if rs.Next() {
			er = rs.Scan(ptr...)
			if er != nil {
				return
			}
		}
		i1 := strings.Index(col[2], "DEFINER")
		i2 := strings.Index(col[2], "TRIGGER")
		if i1 > 0 && i1 < i2 {
			ddl = col[2][:i1] + col[2][i2:]
		} else {
			ddl = col[2]
		}
		return
	}

	err = m.Query(fn, `SHOW CREATE TRIGGER `+trigger)
	return
}

//
func (m *MyConn) Literal(val interface{}, col string) (string, bool) {

	if val == nil {
		return SqlNull, false
	}

	qto, tmf := true, m.Pref.FmtDateTime

	if len(col) > 0 {
		// https://dev.mysql.com/doc/refman/5.7/en/data-types.html
		switch strings.ToUpper(col) {
		case "INTEGER", "INT", "SMALLINT", "TINYINT", "MEDIUMINT", "BIGINT", "DECIMAL", "NUMERIC", "FLOAT", "DOUBLE":
			qto = false
		case "DATE":
			tmf = "2006-01-02"
			qto = true
		case "DATETIME":
			qto = true
		case "TIMESTAMP":
			qto = true
		case "TIME":
			tmf = "15:04:05"
			qto = true
		case "YEAR":
			tmf = "2006"
			qto = true
		case "CHAR", "VARCHAR", "BINARY", "VARBINARY", "BLOB", "TEXT", "ENUM", "SET":
			qto = true
		case "JSON":
			qto = true
		}
	}

	switch v := val.(type) {
	case string:
		return v, qto
	case []byte:
		return string(v), qto
	case sql.NullString:
		if v.Valid {
			return v.String, qto
		} else {
			return SqlNull, false
		}
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), false
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), false
	case float32, float64:
		return fmt.Sprintf("%f", v), false
	case sql.NullBool:
		if v.Valid {
			if v.Bool {
				return SqlTrue, false
			} else {
				return SqlFalse, false
			}
		} else {
			return SqlNull, false
		}
	case sql.NullFloat64:
		if v.Valid {
			return fmt.Sprintf("%f", v.Float64), false
		} else {
			return SqlNull, false
		}
	case sql.NullInt64:
		if v.Valid {
			return fmt.Sprintf("%d", v.Int64), false
		} else {
			return SqlNull, false
		}
	case mysql.NullTime:
		if v.Valid {
			return fmtTime(v.Time, tmf), true
		} else {
			return SqlNull, false
		}
	case time.Time:
		return fmtTime(v, tmf), true
	default:
		return fmt.Sprintf("%v", v), qto
	}
}

func (m *MyConn) Nothing(val interface{}) bool {
	if val == nil {
		return true
	}

	switch v := val.(type) {
	case uint:
		return v <= 0
	case uint8:
		return v <= 0
	case uint16:
		return v <= 0
	case uint32:
		return v <= 0
	case uint64:
		return v <= 0
	case int:
		return v <= 0
	case int8:
		return v <= 0
	case int16:
		return v <= 0
	case int32:
		return v <= 0
	case int64:
	case float32:
		return v <= 0
	case float64:
		return v <= 0
	case string:
		return len(v) == 0
	case []uint8:
		return len(string(v)) == 0
	case sql.NullBool:
		if v.Valid {
			return v.Bool == false
		} else {
			return true
		}
	case sql.NullString:
		if v.Valid {
			return len(v.String) == 0
		} else {
			return true
		}
	case sql.NullFloat64:
		if v.Valid {
			return v.Float64 <= 0
		} else {
			return true
		}
	case sql.NullInt64:
		if v.Valid {
			return v.Int64 <= 0
		} else {
			return true
		}
	case mysql.NullTime:
		if v.Valid {
			return false
		} else {
			return true
		}
	case time.Time:
		return false
	default:
		return len(fmt.Sprintf("%v", v)) == 0
	}
	return false
}

// https://github.com/mysql/mysql-server/blob/mysql-5.7.5/mysys/charset.c#L823-L932
// https://github.com/mysql/mysql-server/blob/mysql-5.7.5/mysys/charset.c#L963-L1038
func (m *MyConn) Quotesc(str, qto string) string {

	ln := len(str)
	var buf strings.Builder
	buf.Grow(ln + ln/20 + 10)

	buf.WriteString(qto)
	for i := 0; i < ln; i++ {
		c := str[i]
		switch c {
		case '\x00':
			buf.WriteByte('\\')
			buf.WriteByte('0')
		case '\n':
			buf.WriteByte('\\')
			buf.WriteByte('n')
		case '\r':
			buf.WriteByte('\\')
			buf.WriteByte('r')
			//case '\x1a':
			//	buf.WriteByte('\\')
			//	buf.WriteByte('Z')
		case '\'':
			buf.WriteByte('\\')
			buf.WriteByte('\'')
		case '"':
			buf.WriteByte('\\')
			buf.WriteByte('"')
		case '\\':
			buf.WriteByte('\\')
			buf.WriteByte('\\')
		default:
			buf.WriteByte(c)
		}
	}
	buf.WriteString(qto)
	return buf.String()
}
