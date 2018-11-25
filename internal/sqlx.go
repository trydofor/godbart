package internal

import "fmt"

const (
	ParaFor = "FOR"
	ParaEen = "END"
	ParaHas = "HAS"
	ParaNot = "NOT"
)

type SqlExe struct {
	Exes []Exe
	Envs map[string]interface{}
}

type Exe struct {
	Seg  *Seg              // 对应的SQL片段
	Stm  string            // 执行的SQL statement,`?`替换
	Args []*Arg            // statement 需要的参数，`？`的顺序
	Envs map[string]string // 依赖的ENV
	Deps map[string]string // 依赖的REF
	Refs map[string]string // 产生的REF
	Fork []*Exe            // 结束时分叉执行
	Done []*Exe            // 结束时执行的RUN
}

func (sqls *SqlSeg) ParseSqlx(envs map[string]string) (exes *SqlExe, err error) {
	return
}

func (sqls *SqlExe) Run(src *MyConn, dst ...*MyConn) {

}

func doExeTree(pref *Preference, segs []Seg, args []Arg) (exes []Exe) {
	// 大部分情况，直接返回
	if len(args) == 0 {
		for _, v := range segs {
			if v.Type != SegCmt {
				exes = append(exes, Exe{&v, v.Text, nil, nil, nil, nil, nil, nil})
			}
		}
		return
	}

	//deep := make(map[string]int) // 某行处SQL段的深度

	for i, v := range segs {
		if v.Type != SegCmt {
			// TODO
			fmt.Printf("\ttodo %d\n", i)
		}
	}
	return
}
