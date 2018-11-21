package internal

import "database/sql"

type Trg struct {
	Name      string
	Timing    string
	Event     string
	Statement string
}

type Idx struct {
	Name string
	Uniq bool
	Cols string
	Type string
}

type Col struct {
	Name string
	Seq  int
	Deft sql.NullString
	Null bool
	Type string
	Key  string
	Cmnt string
	Extr string
}

type Conn interface {
	Open(p *Preference, d *DataSource) (err error)
	DbConn() (db *sql.DB)
	DbName() string
	Exec(qr string, args ...interface{}) (cnt int64, err error)
	Query(fn func(*sql.Rows) error, qr string, args ...interface{}) (err error)
	Tables() (tbls []string, err error)
	Columns(table string) (cls map[string]Col, err error)
	Indexes(table string) (ixs map[string]Idx, err error)
	Triggers(table string) (tgs map[string]Trg, err error)
	DdlTable(table string) (ddl string, err error)
	DdlTrigger(trigger string) (ddl string, err error)
}
