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

func Diff(pref *Preference, dest []DataSource, source *DataSource, tbls []*regexp.Regexp, kind string) (err error) {

	fmt.Println(dest)
	fmt.Println(source)
	return nil
}
