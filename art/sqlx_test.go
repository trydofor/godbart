package art

import (
	"io/ioutil"
	"testing"
)



func Test_ParseSqlx(t *testing.T) {

	//MsgLevel=LvlTrace
	//file := "../demo/sql/tree/test.sql"
	//file := "../demo/sql/tree/tree.sql"
	file := "../demo/sql/tree/stbl.sql"
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

	OutTrace("==== envx ====")
	for k, v := range sqlx.Envs {
		OutTrace("%s=%s", k, v)
	}

	OutTrace("==== exes ====")
	for _, x := range sqlx.Exes {
		OutTrace("%v", x)
	}

	OutTrace("==== summary ====")
	for _, x := range sqlx.Exes {
		OutTrace("%v", x.Tree())
	}
}
