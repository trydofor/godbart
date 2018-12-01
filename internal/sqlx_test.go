package internal

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func Test_ParseSqlx(t *testing.T) {

	file := "../demo/sql/tree/tree.sql"
	//file := "../demo/sql/init/1.table.sql"
	bytes, err := ioutil.ReadFile(file)
	panicIfErr(err)

	sqls, err := ParseSqls(pref, &FileEntity{file, string(bytes)})
	panicIfErr(err)

	envs := make(map[string]string)
	envs["DATE_FROM"] = "2018-11-30 10:31:20"
	sqlx, err := ParseSqlx(sqls, envs)
	panicIfErr(err)

	fmt.Println("args------")
	for _, x := range sqlx.Args {
		fmt.Printf("%v\n", x)
	}

	fmt.Println("exes------")
	for _, x := range sqlx.Exes {
		fmt.Printf("%v\n", x)
	}
}
