package internal

import "database/sql"

const (
	SqlNull  = "NULL"
	SqlTrue  = "TRUE"
	SqlFalse = "FALSE"
)

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
	// 打开链接
	Open(p *Preference, d *DataSource) (err error)
	// 获得链接
	DbConn() (db *sql.DB)
	// 数据库名
	DbName() string

	// 执行脚本
	Exec(qr string, args ...interface{}) (cnt int64, err error)
	// 执行查询
	Query(fn func(*sql.Rows) error, qr string, args ...interface{}) (err error)

	// 获得所有表名
	Tables() (tbls []string, err error)
	// 获得表的所有字段
	Columns(table string) (cls map[string]Col, err error)
	// 获得表的所有索引
	Indexes(table string) (ixs map[string]Idx, err error)
	// 获得表的所有触发器
	Triggers(table string) (tgs map[string]Trg, err error)

	// 生产建表SQL（含索引）
	DdlTable(table string) (ddl string, err error)
	// 生产建触发器SQL
	DdlTrigger(trigger string) (ddl string, err error)

	// 转成SQL字面量，set x=val的 val部分字面量，是否需要引号扩上
	// databaseTypeName sql.ColumnType.DatabaseTypeName
	Literal(val interface{}, databaseTypeName string) (string, bool)
	// 数值<=0|布尔false|NULL|字符串""|其他字面量为""
	Nothing(val interface{}) bool
	// 转义的字符串
	Quotesc(str, qto string) string
}
