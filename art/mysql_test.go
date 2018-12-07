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
	panicIfErr(conn.Open(pref, ddst))

	fmt.Println("database: " + conn.DbName())
	tables, err := conn.Tables()
	panicIfErr(err)

	fmt.Println("tables: " + strings.Join(tables, "\n\t"))

	fmt.Println("columns ------")
	cols, err := conn.Columns("tx_parcel")
	panicIfErr(err)

	for _, v := range cols {
		fmt.Printf("\t%v\n", v)
	}

	fmt.Println("indexes ------")
	idxs, err := conn.Indexes("tx_receiver")
	panicIfErr(err)

	for _, v := range idxs {
		fmt.Printf("\t%v\n", v)
	}

	fmt.Println("trigger ------")
	trgs, err := conn.Triggers("tx_parcel")
	panicIfErr(err)

	for _, v := range trgs {
		fmt.Printf("\t%v\n", v)
	}

	fmt.Println("create table ------")
	ctb, err := conn.DdlTable("tx_parcel$log")
	panicIfErr(err)

	fmt.Println(ctb)

	fmt.Println("create trigger ------")
	ctg, err := conn.DdlTrigger("tx_parcel$log$bu")
	panicIfErr(err)

	fmt.Println(ctg)

	fmt.Println("select args ------")

	var qf = func(rw *sql.Rows) error {
		for rw.Next() {
			var id int64
			rw.Scan(&id)
			fmt.Printf("%d\n", id)
		}
		return nil
	}
	e := conn.Query(qf, "SELECT id FROM tx_parcel WHERE id <= ? and track_num != '??????'", "1163922")
	fmt.Printf("%v\n", e)
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
				fmt.Printf("\ntype=%v, val=%v,sql-type=%#v, literal=%s, quote=%t", reflect.TypeOf(vals[i]), vals[i], types[i].DatabaseTypeName(), s, b)
			}

		}
		return nil
	}, "select * from tx_parcel")

	conn.Query(func(row *sql.Rows) error {
		if row.Next() {
			var ct mysql.NullTime
			row.Scan(&ct)
			s, b := conn.Literal(ct, "")
			fmt.Printf("\ntype=%v, val=%v, literal=%s, quote=%t", reflect.TypeOf(ct), ct, s, b)
		}
		return nil
	}, "select create_time from tx_parcel")
}
