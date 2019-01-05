package art

import (
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"
)

func Test_MakePass(t *testing.T) {
	OutTrace(makePass())
}

func Test_Ctrl_Sync(t *testing.T) {
	CtrlRoom.Open(59062, CtrlRoomTree, nil)
}

func testJob(h, v int, s string) {
	idt := strings.Repeat("| ", v)
	OutTrace("%s<==%d, lvl=%d, at=%s", idt, h, v, s)
	CtrlRoom.dealJobx(nil, h)
}

func mockExe(exe *Exe, lvl int) {

	head := exe.Seg.Head
	jobx := true
	defer func() {
		if jobx {
			testJob(head, lvl, "deref")
		}
	}()

	time.Sleep(time.Second * 3)
	idt := strings.Repeat("| ", lvl)
	if len(exe.Sons) > 0 {
		for i := 0; i < 2; i++ {
			jobx = true
			OutTrace("%sid=%d, lvl=%d, select=%d", idt, head, lvl, i+1)
			for _, v := range exe.Sons {
				mockExe(v, lvl+1)
			}
			jobx = false
			testJob(head, lvl, "for")
		}
	} else {
		OutTrace("%sid=%d, lvl=%d, update", idt, head, lvl)
	}
}

func Test_Ctrl_Mock(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go CtrlRoom.Open(59062, CtrlRoomTree, wg)
	wg.Wait()
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
		OutTrace(e.Tree())
	}
	for {
		for _, v := range sqlx.Exes {
			mockExe(v, 1)
		}
	}
}
