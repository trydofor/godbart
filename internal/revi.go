package internal

import "fmt"

func Revi(pref *Preference, dest []DataSource, file []FileEntity, revi string, test bool) (err error) {
	for _, f := range file {
		fmt.Println(f.Path + "\n" + f.Text)

	}
	fmt.Println(dest)
	fmt.Println(test)
	return nil
}
