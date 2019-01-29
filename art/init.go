package art

const (
	LvlDebug = 300
	LvlTrace = 200
	LvlError = 100

	//
	EnvSrcDb     = "SRC-DB"
	EnvOutDb     = "OUT-DB"
	EnvUser      = "USER"
	EnvHost      = "HOST"
	EnvDate      = "DATE"
	EnvRule      = "ENV-CHECK-RULE"
	EnvRuleEmpty = "EMPTY"

	//
	SqlNull  = "NULL"
	SqlTrue  = "TRUE"
	SqlFalse = "FALSE"

	//
	DiffSum = "sum" // 分别对比`-s`和多个`-d` 间的表名差异
	DiffTrg = "trg" // 比较 trigger
	DiffTbl = "tbl" // 比较 column, index

	//
	Joiner = "\n"

	//
	SyncTbl = "tbl" // 同步表和索引
	SyncTrg = "trg" // 同步trigger
	SyncRow = "row" // 同步数据
)

var (
	MsgLevel = LvlDebug
	DiffType = map[string]bool{DiffSum: true, DiffTrg: true, DiffTbl: true}
	SyncType = map[string]bool{SyncTbl: true, SyncTrg: true, SyncRow: true}
	EmptyArr = make([]interface{}, 0)
	CtrlRoom = &Room{}
)
