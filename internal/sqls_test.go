package internal

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func Test_ParseSql(t *testing.T) {

	file := "../demo/sql/mig/tree.sql"
	//file := "../demo/sql/img/1.table.sql"
	bytes, err := ioutil.ReadFile(file)
	panicIfErr(err)

	sqld, err := ParseSql(pref, &FileEntity{file, string(bytes)})
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
