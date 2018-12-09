package art

import "testing"

func Test_Exec(t *testing.T) {
	file := makeFileEntity("../demo/sql/init/1.table.sql", "../demo/sql/init/2.data.sql")
	Exec(pref, dsts, file, false)
}
