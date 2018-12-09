package art

import (
	"fmt"
	"io/ioutil"
	"testing"
)



func Test_ParseSqlx(t *testing.T) {

	//file := "../demo/sql/tree/test.sql"
	file := "../demo/sql/tree/tree.sql"
	//file := "../demo/sql/init/1.table.sql"
	bytes, err := ioutil.ReadFile(file)
	panicIfErr(err)

	sqls := ParseSqls(pref, &FileEntity{file, string(bytes)})

	envs := make(map[string]string)
	envs["DATE_FROM"] = "2018-11-30 10:31:20"
	envs["带空格的 时间"] = "2018-11-30 10:31:20"
	BuiltinEnvs(envs)

	sqlx, err := ParseSqlx(sqls, envs)
	panicIfErr(err)

	fmt.Println("==== envx ====")
	for k, v := range sqlx.Envs {
		fmt.Printf("%s=%s\n", k, v)
	}

	fmt.Println("==== exes ====")
	for _, x := range sqlx.Exes {
		fmt.Printf("%v\n", x)
	}
}
