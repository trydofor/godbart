package internal

import "fmt"

func Revi(pref *Preference, dest []DataSource, sqls []FileEntity, revi string, test bool) (err error) {
	for _, f := range sqls {
		fmt.Println(f.Path + "\n" + f.Text)

	}
	fmt.Println(dest)
	fmt.Println(test)
	return nil
}
