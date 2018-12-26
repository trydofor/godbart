package art

const (
	LvlDebug = 300
	LvlTrace = 200
	LvlError = 100

	//
	EnvUser      = "USER"
	EnvHost      = "HOST"
	EnvDate      = "DATE"
	EnvRule      = "ENV-CHECK-RULE"
	EnvRuleError = "ERROR"
	EnvRuleEmpty = "EMPTY"

	//
	SqlNull  = "NULL"
	SqlTrue  = "TRUE"
	SqlFalse = "FALSE"

	//
	DiffTbl = "tbl" // 分别对比`-s`和多个`-d` 间的表名差异
	DiffAll = "all" // 分别对比`-s`和多个`-d` 间的表明细(column, index,trigger)
	DiffDdl = "ddl" // 生成多库的创建DDL(table&index，trigger)

	//
	Joiner = "\n"

	//
	SyncTbl = "tbl" // 同步表和索引
	SyncAll = "all" // 同步说有
	SyncTrg = "trg" // 同步trigger
)

var (
	MsgLevel = LvlDebug
	DiffType = []string{DiffTbl, DiffAll, DiffDdl}
	SyncType = []string{SyncTbl, SyncAll, SyncTrg}
	EmptyArr = []interface{}{}
	CtrlRoom = &Room{}
)
