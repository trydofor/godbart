package art

import "testing"

func Test_Exec(t *testing.T) {
	file := makeFileEntity("../demo/sql/diff/reset.sql", "../demo/sql/tree/test.sql")
	Exec(pref, dsts, file, false)
}
