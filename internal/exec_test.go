package internal

import "testing"

func Test_Exec(t *testing.T) {
	file := makeFileEntity("../demo/sql/diff/reset.sql")
	Exec(pref, dsts, file, false)
}
