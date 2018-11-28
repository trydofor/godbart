package internal

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func Test_ParseSql(t *testing.T) {

	file := "../demo/sql/tree/tree.sql"
	//file := "../demo/sql/init/1.table.sql"
	bytes, err := ioutil.ReadFile(file)
	panicIfErr(err)

	sqld, err := ParseSqls(pref, &FileEntity{file, string(bytes)})
	panicIfErr(err)

	fmt.Println("args------")
	for _, x := range sqld.Args {
		fmt.Printf("%#v\n", x)
	}
	fmt.Println("segs------")
	for _, x := range sqld.Segs {
		fmt.Printf("%#v\n", x)
	}
}

func Test_DepairQuote(t *testing.T) {

	q2 := "`'12345'`"
	fmt.Printf("%s\n", q2)

	cnt := countQuotePair(q2)
	fmt.Printf("%d\n", cnt)
}