package internal

import (
	"io/ioutil"
)

var (
	pref = &Preference{"mysql", ";", "DELIMITER", "--", []string{"/*", "*/"}, "2006-01-02 15:04:05.000", 10, 2}
	dsrc = &DataSource{"prd_main", "trydofor:moilioncircle@tcp(127.0.0.1:3306)/prd_main"}
	ddst = &DataSource{"prd_2018", "trydofor:moilioncircle@tcp(127.0.0.1:3306)/prd_2018"}
	dsts = []*DataSource{ddst}
	mask = "[0-9]{10,}"
)

func makeFileEntity(p string) []FileEntity {
	data, err := ioutil.ReadFile(p)
	panicIfErr(err)
	return []FileEntity{{p, string(data)}}
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
