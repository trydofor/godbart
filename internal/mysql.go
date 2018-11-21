package internal

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"strings"
)

type MyConn struct {
	Conn *sql.DB
	Name string
}

func (m *MyConn) Open(p *Preference, d *DataSource) (err error) {
	if p.DatabaseType != "mysql" {
		err = errors.New("unsupported DatabaseType, need mysql, but " + p.DatabaseType)
		return
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

func (m *MyConn) Exec(qr string, args ...interface{}) (cnt int64, err error) {

	rs, err := m.Conn.Exec(qr, args...)
	if err != nil {
		return
	}

	cnt, err = rs.RowsAffected()
	return
}

func (m *MyConn) Query(fn func(*sql.Rows) error, qr string, args ...interface{}) (err error) {
	rs, err := m.Conn.Query(qr, args...)
	if err != nil {
		return
	}
	defer rs.Close()

	err = fn(rs)
	return
}

func (m *MyConn) Tables() (tbls []string, err error) {
	var sn = func(rs *sql.Rows) (err error) {
		for rs.Next() {
			var val string
			err = rs.Scan(&val)
			if err != nil {
				return
			}
			tbls = append(tbls, val)
		}
		return
	}
	err = m.Query(sn, `SHOW TABLES`)
	return
}

func (m *MyConn) Columns(table string) (cls map[string]Col, err error) {
	var sn = func(rs *sql.Rows) (err error) {
		cls = make(map[string]Col)
		for rs.Next() {
			var cl Col
			var nl string
			err = rs.Scan(&cl.Name, &cl.Seq, &cl.Deft, &nl, &cl.Type, &cl.Key, &cl.Cmnt, &cl.Extr)
			if err != nil {
				return
			}
			cl.Null = strings.EqualFold(nl, "YES")
			cls[cl.Name] = cl
		}
		return
	}
	err = m.Query(sn, `
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
	return
}

func (m *MyConn) Indexes(table string) (ixs map[string]Idx, err error) {
	var sn = func(rs *sql.Rows) (err error) {
		ixs = make(map[string]Idx)
		for rs.Next() {
			var ix Idx
			var nq int
			err = rs.Scan(&ix.Name, &nq, &ix.Cols, &ix.Type)
			if err != nil {
				return
			}
			ix.Uniq = nq == 0
			ixs[ix.Name] = ix
		}
		return
	}

	err = m.Query(sn, `
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
	return
}

func (m *MyConn) Triggers(table string) (tgs map[string]Trg, err error) {
	var sn = func(rs *sql.Rows) (err error) {
		tgs = make(map[string]Trg)
		for rs.Next() {
			var tg Trg
			err = rs.Scan(&tg.Name, &tg.Timing, &tg.Event, &tg.Statement)
			if err != nil {
				return
			}
			tgs[tg.Name] = tg
		}
		return
	}
	err = m.Query(sn, `
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
	return
}

func (m *MyConn) DdlTable(table string) (ddl string, err error) {
	var sn = func(rs *sql.Rows) (err error) {
		var nm string
		if rs.Next() {
			err = rs.Scan(&nm, &ddl)
			if err != nil {
				return
			}
		}
		ddl = fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n%s\n", table, ddl)
		return
	}
	err = m.Query(sn, `SHOW CREATE TABLE `+table)

	return
}

func (m *MyConn) DdlTrigger(trigger string) (ddl string, err error) {
	var sn = func(rs *sql.Rows) (err error) {
		var col = make([]string, 7)
		var ptr = make([]interface{}, 7)
		for i, _ := range col {
			ptr[i] = &col[i]
		}
		if rs.Next() {
			err = rs.Scan(ptr...)
			if err != nil {
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
		ddl = fmt.Sprintf("DROP TRIGGER IF EXISTS `%s`;\nDELIMITER $$\n%s $$\nDELIMITER ;\n", trigger, ddl)
		return
	}
	err = m.Query(sn, `SHOW CREATE TRIGGER `+trigger)
	return
}
