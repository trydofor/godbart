package internal

var (
	pref = &Preference{"mysql", ";", "DELIMITER", true, false, "--", []string{"/*", "*/"}, 10, 2}
	dest = &DataSource{"prd_2018", "trydofor:moilioncircle@tcp(127.0.0.1:3306)/prd_2018"}
)

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}
