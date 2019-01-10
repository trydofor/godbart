package art

import (
	"testing"
)

func Test_Tree(t *testing.T) {

	//MsgLevel = LvlTrace
	envs := make(map[string]string)
	envs["DATE_FROM"] = "2018-01-01 01:01:01"
	envs[EnvRule] = EnvRuleEmpty
	BuiltinEnvs(envs)
	//file := makeFileEntity("../demo/sql/tree/tree.sql")
	//file := makeFileEntity("../demo/sql/tree/renv.sql")
	file := makeFileEntity("../demo/sql/tree/stbl.sql")
	Tree(pref, envs, dsrc, dstt, file, true)
}
