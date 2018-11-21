package internal

import "fmt"

func Diff(pref *Preference, dest []DataSource, source *DataSource, sync *DiffSchema, test bool) (err error) {

	fmt.Println(dest)
	fmt.Println(source)
	fmt.Println(test)
	return nil
}
