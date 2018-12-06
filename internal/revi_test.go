package internal

import "testing"

func Test_Revi(t *testing.T) {

	file := makeFileEntity("../demo/sql/revi/2018-11-20.sql")
	Revi(pref, dsts, file, "2018112001", mask, false)
}
