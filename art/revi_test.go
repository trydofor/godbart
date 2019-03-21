package art

import "testing"

func Test_Revi(t *testing.T) {

	//file := makeFileEntity("../demo/sql/revi/2018-11-18.sql", "../demo/sql/revi/2018-11-20.sql")
	file := makeFileEntity("../demo/sql/revi/2019-01-11.sql")
	Revi(pref, dsts, file, "2019030601", mask, "",false)
}
