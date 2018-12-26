package art

import (
	"io/ioutil"
	"testing"
)

func Test_ParseSql(t *testing.T) {

	//file := "../demo/sql/tree/test.sql"
	//file := "../demo/sql/tree/tree.sql"
	file := "../demo/sql/init/2.data.sql"

	bytes, err := ioutil.ReadFile(file)
	panicIfErr(err)

	sqls := ParseSqls(pref, &FileEntity{file, string(bytes)})

	OutTrace("segs------")
	for _, x := range sqls {
		OutTrace("%#v", x)
	}
}

func Test_DepairQuote(t *testing.T) {

	q2 := "`'12345'`"
	OutTrace("%s", q2)

	cnt := countQuotePair(q2)
	OutTrace("%d", cnt)
}
