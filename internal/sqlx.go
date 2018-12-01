package internal

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
)

const (
	//
	CmndEnv = "ENV"
	CmndRef = "REF"
	CmndStr = "STR"
	CmndRun = "RUN"
	CmndOut = "OUT"

	//
	ParaFor = "FOR"
	ParaEen = "END"
	ParaHas = "HAS"
	ParaNot = "NOT"
)

var cmdArrs = []string{CmndEnv, CmndRef, CmndStr, CmndRun, CmndOut}

var argsReg = regexp.MustCompile(`(?i)` + // 不区分大小写
	`^[^0-9A-Z]*` + // 非英数开头，视为注释部分
	`(` + strings.Join(cmdArrs, "|") + `)[ \t]+` + //命令和空白，第一分组，固定值
	"([^`'\" \t]+|'[^']+'|\"[^\"]+\"|`[^`]+`)[ \t]+" + // 变量和空白，第二分组，
	"([^`'\" \t]+|'[^']+'|\"[^\"]+\"|`[^`]+`)") // 连续的非引号空白或，引号成对括起来的字符串（贪婪）

type SqlExe struct {
	Envs map[string]string // 静态环境变量。不放动态运行时
	Args map[string]*Arg   // hold和Arg关系
	Exes []*Exe            // 数据树
}

type Exe struct {
	Seg Sql // 对应的SQL片段

	// 产出
	Refs []*Arg // 提取REF
	Strs []*Arg // 静态STR
	// 行为
	Runs []*Arg // 源库RUN
	Outs []*Arg // 它库OUT

	// 依赖，顺序重要
	Deps []*Hld // 依赖

	// 运行，深度优先L
	Sons []*Exe // 分叉
}

type Arg struct {
	Line string // 开始和结束行，全闭区间
	Head int    // 首行
	Type string // 参数类型 Cmnd*
	Para string // 变量名
	Hold string // 占位符
}

type Hld struct {
	Idx int    // 开始位置，包含
	End int    // 结束位置，不包括
	Str string // HOLD字符串
	Arg *Arg   // 对应的ARG
}

func (x Exe) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("\n{%p\nSql:%#v", &x, x.Seg))

	if len(x.Refs) > 0 {
		sb.WriteString(" \nRefs:[")
		for _, v := range x.Refs {
			sb.WriteString(fmt.Sprintf("\n   %#v", *v))
		}
		sb.WriteString("]")
	}
	if len(x.Strs) > 0 {
		sb.WriteString(" \nStrs:[")
		for _, v := range x.Strs {
			sb.WriteString(fmt.Sprintf("\n   %#v", *v))
		}
		sb.WriteString("]")
	}
	if len(x.Runs) > 0 {
		sb.WriteString(" \nRuns:[")
		for _, v := range x.Runs {
			sb.WriteString(fmt.Sprintf("\n   %#v", *v))
		}
		sb.WriteString("]")
	}
	if len(x.Outs) > 0 {
		sb.WriteString(" \nOuts:[")
		for _, v := range x.Outs {
			sb.WriteString(fmt.Sprintf("\n   %#v", *v))
		}
		sb.WriteString("]")
	}
	if len(x.Deps) > 0 {
		sb.WriteString(" \nDeps:[")
		for _, v := range x.Deps {
			sb.WriteString(fmt.Sprintf("\n   internal.Hld{Idx:%d, End:%d, Str:%q, Arg:%#v]", v.Idx, v.End, v.Str, v.Arg))
		}
		sb.WriteString("]")
	}
	if len(x.Sons) > 0 {
		sb.WriteString(" \nSons:[")
		for _, v := range x.Sons {
			son := fmt.Sprintf("%v", v)
			sb.WriteString(fmt.Sprintf("%s", strings.Replace(son, "\n", "\n   |    ", -1)))
		}
		sb.WriteString("]")
	}
	sb.WriteString("\n}\n")
	return sb.String()
}

func ParseSqlx(sqls []Sql, envs map[string]string) (sqlx *SqlExe, err error) {

	// 除了静态环境变量，都是运行时确定的。
	holdExe := make(map[string]*Exe)   // hold出生的Exe
	holdCnt := make(map[string]int)    // HOLD引用计数
	lineArg := make(map[string][]*Arg) // 语句块和ARG

	var root []*Exe
	args := make(map[string]*Arg) // hold所在的Arg

	iarg := -1
	for i, seg := range sqls {
		// 解析指令
		if seg.Type == SegCmt {
			if iarg < 0 {
				iarg = i
			}
			ags := parseArgs(seg.Text, seg.Head)
			for _, k := range ags {
				lineArg[seg.Line] = append(lineArg[seg.Line], k)
				// 定义指令，检测重复
				if k.Type == CmndEnv || k.Type == CmndRef || k.Type == CmndStr {
					od, ok := args[k.Hold]
					if ok {
						s := fmt.Sprintf("duplicate HOLD=%s, line1=%s, line2=%s, file=%s", k.Hold, od.Line, k.Line, seg.File)
						log.Fatalf("[ERROR] %s\n", s)
						return nil, errors.New(s)
					}
					args[k.Hold] = k
					holdCnt[k.Hold] = 0
				}

				// 环境变量检查
				if k.Type == CmndEnv {
					_, ok := envs[k.Para]
					if !ok {
						s := fmt.Sprintf("ENV not found. para=%s, line=%s, file=%s", k.Para, k.Line, seg.File)
						log.Fatalf("[ERROR] %s\n", s)
						return nil, errors.New(s)
					}
				}
				//fmt.Printf("p0:%#v\n", k)
			}
			continue
		}

		exe := &Exe{}
		exe.Seg = seg

		// 挂靠
		if iarg >= 0 { //有注释的
			for j := iarg; j < i; j++ {
				arg, ok := lineArg[sqls[j].Line]
				if ok {
					for _, k := range arg {
						switch k.Type {
						case CmndRef:
							exe.Refs = append(exe.Refs, k)
							holdExe[k.Hold] = exe
						case CmndStr:
							rg, hz := args[k.Para] // 重定义
							if hz {
								if rg.Type == CmndEnv {
									// ignore
								} else {
									ex, kx := holdExe[k.Para]
									if !kx {
										s := fmt.Sprintf("STR redefine REF not found. para=%s, line=%s, file=%s", k.Para, k.Line, seg.File)
										log.Fatalf("[ERROR] %s\n", s)
										return nil, errors.New(s)
									}
									ex.Strs = append(ex.Strs, k)
									holdExe[k.Hold] = ex
								}
							} else {
								exe.Strs = append(exe.Strs, k)
								holdExe[k.Hold] = exe
							}
						case CmndRun:
							exe.Runs = append(exe.Runs, k)
						case CmndOut:
							exe.Outs = append(exe.Outs, k)
						}
					}
				}
			}
			iarg = -1
		}

		// 分析HOLD依赖
		var deps []*Hld // HOLD依赖
		stmt := exe.Seg.Text
		for hd := range args {
			off, lln := 0, len(hd)
			for {
				p := strings.Index(stmt[off:], hd)
				if p < 0 {
					break
				}

				// 引用计数
				holdCnt[hd] = holdCnt[hd] + 1
				// 解析依赖
				off = p + lln // 更新位置
				deps = append(deps, &Hld{p, off + 1, hd, args[hd]})
			}
		}
		sort.Slice(deps, func(i, j int) bool {
			return deps[i].Idx < deps[j].Idx
		})
		exe.Deps = deps

		// 深度优先寻父。
		top := true
		for _, v := range exe.Runs {
			pa, ok := holdExe[v.Hold]
			if ok {
				pa.Sons = append(pa.Sons, exe)
				top = false
			} else {
				s := fmt.Sprintf("RUN HOLD's REF not found, hold=%s, line=%s, file=%s", v.Hold, v.Line, seg.File)
				log.Fatalf("[ERROR] %s\n", s)
				return nil, errors.New(s)
			}
		}

		for _, v := range exe.Outs {
			pa, ok := holdExe[v.Hold]
			if ok {
				pa.Sons = append(pa.Sons, exe)
				top = false
			} else {
				s := fmt.Sprintf("OUT HOLD's REF not found, hold=%s, line=%s, file=%s", v.Hold, v.Line, seg.File)
				log.Fatalf("[ERROR] %s\n", s)
				return nil, errors.New(s)
			}
		}

		if top {
			for _, v := range exe.Deps {
				pa, ok := holdExe[v.Str]
				if ok { // // REF|STR HOLD
					if !top {
						s := fmt.Sprintf("REF HOLD's multiple found, hold=%s, line=%s, file=%s", v.Arg.Hold, v.Arg.Line, seg.File)
						log.Fatalf("[ERROR] %s\n", s)
						return nil, errors.New(s)
					}
					pa.Sons = append(pa.Sons, exe)
					top = false
				}
			}
		}
		//fmt.Printf("p1:%v\n", exe)
		if top {
			root = append(root, exe)
		}
	}

	// 清理无用REF
	for k, c := range holdCnt {
		if c == 0 {
			e, ok := holdExe[k]
			if !ok { // 重定义
				continue
			}
			p1, p2 := -1, -1
			for i, v := range e.Refs {
				if v.Hold == k {
					p1 = i
					break
				}
			}
			for i, v := range e.Strs {
				if v.Hold == k {
					p2 = i
					break
				}
			}
			if p1 >= 0 {
				log.Printf("[TRACE] remove unused REF=%#v\n", e.Refs[p1])
				e.Refs = append(e.Refs[:p1], e.Refs[p1+1:]...)
			}
			if p2 >= 0 {
				log.Printf("[TRACE] remove unused STR=%#v\n", e.Strs[p2])
				e.Strs = append(e.Strs[:p2], e.Strs[p2+1:]...)
			}

		}
	}

	sqlx = &SqlExe{envs, args, root}
	return
}

func (sqlx *SqlExe) Run(src *MyConn, dst ...*MyConn) {
	//dynEnv := make(map[string]interface{}) // 存放select的REF

}

func parseArgs(text string, h int) (args []*Arg) {
	// 分析参数 ENV REF RUN
	line := strings.Split(text, Joiner)
	ln := fmt.Sprintf("%d:%d", h, h+len(line)-1)

	for i, v := range line {
		sm := argsReg.FindStringSubmatch(v)
		if len(sm) == 4 {
			cmd := strings.ToUpper(sm[1])
			if cmd == CmndRun || cmd == CmndOut {
				sm[2] = strings.ToUpper(sm[2]) // 命令变量大写
			} else if cmd == CmndRef || cmd == CmndEnv {
				if cp := countQuotePair(sm[2]); cp > 0 { //脱去最外层引号
					sm[2] = sm[2][1 : len(sm[2])-1]
				}
			} else if cmd == CmndStr {
				pq := countQuotePair(sm[2])
				hq := countQuotePair(sm[3])
				if pq > 0 && hq > 0 && sm[2][0] == sm[3][0] {
					sm[2] = sm[2][1 : len(sm[2])-1]
					sm[3] = sm[3][1 : len(sm[3])-1]
				}
			}

			arg := &Arg{ln, i + h, cmd, sm[2], sm[3]}
			args = append(args, arg)
		}
	}
	return
}

func doExeTree(pref *Preference, segs []Sql, args []Arg) (exes []*Exe) {
	// 大部分情况，直接返回
	if len(args) == 0 {
		for _, v := range segs {
			if v.Type != SegCmt {
				exes = append(exes, nil)
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
