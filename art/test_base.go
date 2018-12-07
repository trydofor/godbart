package art

import (
	"io/ioutil"
)

var (
	pref = &Preference{"mysql", ";", "DELIMITER", "--", []string{"/*", "*/"}, "2006-01-02 15:04:05.000", 10, 2}
	dsrc = &DataSource{"prd_main", "trydofor:moilioncircle@tcp(127.0.0.1:3306)/godbart_prd_main"}
	ddst = &DataSource{"prd_2018", "trydofor:moilioncircle@tcp(127.0.0.1:3306)/godbart_prd_2018"}
	dsts = []*DataSource{ddst}
	mask = "[0-9]{10,}"
)

func makeFileEntity(file ...string) []FileEntity {
	rst := make([]FileEntity, len(file))
	for i, f := range file {
		data, err := ioutil.ReadFile(f)
		panicIfErr(err)
		rst[i] = FileEntity{f, string(data)}
	}
	return rst
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
