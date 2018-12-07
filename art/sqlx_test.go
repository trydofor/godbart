package art

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
)

func (x Exe) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("\n{\nSql:%#v", x.Seg))

	if len(x.Defs) > 0 {
		sb.WriteString(" \nDefs:[")
		for h, p := range x.Defs {
			sb.WriteString(fmt.Sprintf("\n   hold:%s, para:%s", h, p))
		}
		sb.WriteString("]")
	}

	if len(x.Acts) > 0 {
		sb.WriteString(" \nActs:[")
		for _, v := range x.Acts {
			sb.WriteString(fmt.Sprintf("\n   %#v", *v))
		}
		sb.WriteString("]")
	}
	if len(x.Deps) > 0 {
		sb.WriteString(" \nDeps:[")
		for _, v := range x.Deps {
			sb.WriteString(fmt.Sprintf("\n   %#v", v))
		}
		sb.WriteString("]")
	}
	if len(x.Sons) > 0 {
		sb.WriteString(" \nSons:[")
		for _, v := range x.Sons {
			son := fmt.Sprintf("%v", v)
			sb.WriteString(fmt.Sprintf("%s", strings.Replace(son, "\n", "\n   |    ", -1)))
		}
		sb.WriteString("]")
	}
	sb.WriteString("\n}")
	return sb.String()
}

func Test_ParseSqlx(t *testing.T) {

	file := "../demo/sql/tree/test.sql"
	//file := "../demo/sql/tree/tree.sql"
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
