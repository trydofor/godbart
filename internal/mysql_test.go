package internal

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func TestMyConn(t *testing.T) {

	conn := MyConn{}
	err := conn.Open(pref, dest)
	panicIfErr(err)

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
