package internal

import "testing"

func Test_Tree(t *testing.T) {

	envs := make(map[string]string)
	envs["DATE_FROM"] = "2018-02-02 02:02:02"
	envs["带空格的 时间"] = "2018-01-01 01:01:01"
	BuiltinEnvs(envs)
	file := makeFileEntity("../demo/sql/tree/test.sql")
	Tree(pref, envs, dsrc, dsts, file, false)
}
