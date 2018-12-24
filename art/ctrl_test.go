package art

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"
)

func Test_MakePass(t *testing.T) {
	fmt.Println(makePass())
}

func Test_Ctrl_Sync(t *testing.T) {
	CtrlRoom.Open(59062, "tree")
}

func testWalk(exe *Exe) {

	head := exe.Seg.Head
	defer func() {
		CtrlRoom.dealJobx(nil, head)
	}()

	time.Sleep(time.Second * 3)
	fmt.Printf("id=%d, sleep 3 seconds\n", head)
	for _, v := range exe.Sons {
		// 都是三条记录
		testWalk(v)
		testWalk(v)
		testWalk(v)
	}
}

func Test_Ctrl_Mock(t *testing.T) {
	go CtrlRoom.Open(59062, "tree")

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

	CtrlRoom.putEnv(roomTreeEnvSqlx, sqlx)
	for _, e := range sqlx.Exes {
		fmt.Println(e.Tree())
	}
	for {
		for _, v := range sqlx.Exes {
			testWalk(v)
		}
	}
}
