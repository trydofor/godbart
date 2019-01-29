// +build database

package art

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"reflect"
	"strings"
	"testing"
)

func Test_MyConn(t *testing.T) {

	conn := MyConn{}
	panicIfErr(conn.Open(pref, dsrc))

	OutTrace("database: " + conn.DbName())
	tables, err := conn.Tables()
	panicIfErr(err)

	OutTrace("tables: " + strings.Join(tables, "\n\t"))

	OutTrace("columns ------")
	cols, err := conn.Columns("tx_parcel")
	panicIfErr(err)

	for _, v := range cols {
		OutTrace("\t%v", v)
	}

	OutTrace("indexes ------")
	idxs, err := conn.Indexes("tx_receiver")
	panicIfErr(err)

	for _, v := range idxs {
		OutTrace("\t%v", v)
	}

	OutTrace("trigger ------")
	trgs, err := conn.Triggers("tx_parcel")
	panicIfErr(err)

	for _, v := range trgs {
		OutTrace("\t%v", v)
	}

	OutTrace("create table ------")
	ctb, err := conn.DdlTable("tx_parcel$log")
	panicIfErr(err)

	OutTrace(ctb)

	OutTrace("create trigger ------")
	ctg, err := conn.DdlTrigger("tx_parcel$log$bu")
	panicIfErr(err)

	OutTrace(ctg)

	OutTrace("select args ------")

	var qf = func(rw *sql.Rows) error {
		for rw.Next() {
			var id int64
			rw.Scan(&id)
			OutTrace("%d", id)
		}
		return nil
	}
	e := conn.Query(qf, "SELECT id FROM tx_parcel WHERE id <= ? and track_num != '??????'", "1163922")
	OutTrace("%v", e)
}

func Test_Query(t *testing.T) {
	conn := MyConn{}
	panicIfErr(conn.Open(pref, dsrc))

	conn.Query(func(row *sql.Rows) error {
		if row.Next() {
			types, e := row.ColumnTypes()
			if e != nil {
				return e
			}
			ln := len(types)
			vals := make([]interface{}, ln)
			ptrs := make([]interface{}, ln)
			for i := 0; i < ln; i++ {
				ptrs[i] = &vals[i]
			}

			row.Scan(ptrs...)
			for i := 0; i < ln; i++ {
				s, b := conn.Literal(vals[i], types[i].DatabaseTypeName())
				OutTrace("type=%v, val=%v,sql-type=%#v, literal=%s, quote=%t", reflect.TypeOf(vals[i]), vals[i], types[i].DatabaseTypeName(), s, b)
			}

		}
		return nil
	}, "select * from tx_parcel")

	conn.Query(func(row *sql.Rows) error {
		if row.Next() {
			var ct mysql.NullTime
			row.Scan(&ct)
			s, b := conn.Literal(ct, "")
			OutTrace("type=%v, val=%v, literal=%s, quote=%t", reflect.TypeOf(ct), ct, s, b)
		}
		return nil
	}, "select create_time from tx_parcel")
}

func Test_Mdb(t *testing.T) {
	conn := MyConn{}
	panicIfErr(conn.Open(pref, dsrc))

	i, e := conn.Exec(`replace into godbart_prd_2018.sys_schema_version select * from sys_schema_version`)
	fmt.Printf("%d, %#v", i, e)

}
