package internal

import (
	"fmt"
	"regexp"
)

const (
	TbName = "tbname"
	Detail = "detail"
	Create = "create"
)

var DiffKinds = []string{TbName, Detail, Create}

func Diff(pref *Preference, dest []DataSource, source *DataSource, kind string, tbls []*regexp.Regexp) (err error) {

	fmt.Println(dest)
	fmt.Println(source)
	return nil
}
