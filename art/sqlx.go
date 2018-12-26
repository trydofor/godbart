package art

import (
	"fmt"
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
	ParaOne = "ONE"
	ParaEnd = "END"
	ParaHas = "HAS"
	ParaNot = "NOT"
)

var cmdArrs = []string{CmndEnv, CmndRef, CmndStr, CmndRun, CmndOut}
var paraWgt = []string{ParaOne, ParaFor, ParaEnd} // `REF`<`ONE`<`FOR`<`END`

var argsReg = regexp.MustCompile(`(?i)` + // 不区分大小写
	`^[^0-9A-Z]*` + // 非英数开头，视为注释部分
	`(` + strings.Join(cmdArrs, "|") + `)[ \t]+` + //命令和空白，第一分组，固定值
	"([^`'\" \t]+|'[^']+'|\"[^\"]+\"|`[^`]+`)[ \t]+" + // 变量和空白，第二分组，
	"([^`'\" \t]+|'[^']+'|\"[^\"]+\"|`[^`]+`)") // 连续的非引号空白或，引号成对括起来的字符串（贪婪）

type SqlExe struct {
	Envs map[string]string // HOLD对应的环境变量
	Exes []*Exe            // 数据树
}

type Exe struct {
	Seg Sql // 对应的SQL片段

	// 产出
	Defs map[string]string // REF|STR的提取，key=HOLD,val=PARA
	// 行为
	Acts []*Arg // 源库RUN & OUT

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
	Off int    // 开始位置，包含
	End int    // 结束位置，包括
	Ptn string // PARA的模式
	Str string // HOLD字符串
	Dyn bool   // true：动态替换；false：静态替换
}

func ParseSqlx(sqls Sqls, envs map[string]string) (*SqlExe, error) {
	logDebug("build a SQLX")

	// 除了静态环境变量，都是运行时确定的。
	holdExe := make(map[string]*Exe)   // hold出生的Exe
	holdCnt := make(map[string]int)    // HOLD引用计数
	holdStr := make(map[string]bool)   // HOLD为STR指令的
	lineArg := make(map[string][]*Arg) // 语句块和ARG

	var tops, alls []*Exe
	argx := make(map[string]*Arg)   // hold对应的Arg
	envx := make(map[string]string) // hold对应的ENV

	sonFunc := func(prn, exe *Exe, top *bool) {
		h := true

		for i := len(prn.Sons) - 1; i >= 0; i-- {
			if prn.Sons[i].Seg.Head == exe.Seg.Head {
				h = false
				break
			}
		}

		if h {
			prn.Sons = append(prn.Sons, exe)
		}
		*top = false
	}

	iarg := -1
	for i, seg := range sqls {
		// 解析指令
		if !seg.Exeb {
			if iarg < 0 {
				iarg = i
			}
			ags := parseArgs(seg.Text, seg.Head)
			for _, gx := range ags {
				lineArg[seg.Line] = append(lineArg[seg.Line], gx)
				// 定义指令，检测重复
				logDebug("parsed Arg=%#v", gx)
				if gx.Type == CmndEnv || gx.Type == CmndRef || gx.Type == CmndStr {
					od, ok := argx[gx.Hold]
					if ok {
						return nil, errorAndLog("duplicate HOLD=%s, line1=%d, line2=%d, file=%s", gx.Hold, od.Head, gx.Head, seg.File)
					}
					argx[gx.Hold] = gx
					holdCnt[gx.Hold] = 0
				}
			}
			continue
		}

		exe := &Exe{}
		exe.Seg = seg

		logDebug("build an Exe, line=%s, file=%s", seg.Line, seg.File)

		// 挂靠
		if iarg >= 0 { //有注释的
			defs := make(map[string]string)
			rule := envs[EnvRule]
			for j := iarg; j < i; j++ {
				if arg, ok := lineArg[sqls[j].Line]; ok {
					for _, gx := range arg {
						switch gx.Type {
						case CmndEnv:
							if ev, kx := envs[gx.Para]; !kx {
								if rule == EnvRuleError {
									return nil, errorAndLog("ENV not found. para=%s, line=%d, file=%s", gx.Para, gx.Head, seg.File)
								} else {
									envx[gx.Hold] = ""
									logDebug("checked def ENV, set Empty, Arg's line=%d, para=%s", gx.Head, gx.Para)
								}
							} else {
								envx[gx.Hold] = ev
								logDebug("checked def ENV, Arg's line=%d, para=%s, env=%s", gx.Head, gx.Para, ev)
							}
						case CmndRef:
							defs[gx.Hold] = gx.Para
							holdExe[gx.Hold] = exe
							logDebug("appended Exe's REF, Arg's line=%d, para=%s, hold=%s", gx.Head, gx.Para, gx.Hold)
						case CmndStr:
							holdStr[gx.Hold] = true
							hd := gx.Para
							var rg *Arg // 追到源头，是否重定义

							for tm, hz := argx[hd]; hz; tm, hz = argx[hd] {
								hd = tm.Para
								rg = tm
							}

							if rg == nil { // 直接定义
								if ev, kx := envs[gx.Para]; kx { //ENV
									envx[gx.Hold] = ev
									logDebug("checked STR def ENV, Arg's line=%d, para=%s, env=%s", gx.Head, gx.Para, ev)
								} else { // REF
									holdExe[gx.Hold] = exe
									defs[gx.Hold] = gx.Para
									logDebug("appended Exe's STR def REF, Arg's line=%d, hold=%s", gx.Head, gx.Hold)
								}
							} else { // 重新定义
								if rg.Type == CmndEnv { // 重定义的ENV
									if ev, kx := envs[rg.Para]; kx {
										envx[gx.Hold] = ev
										logDebug("checked STR redef ENV, Arg's line=%d, para=%s, env=%s", gx.Head, rg.Para, ev)
									} else {
										if rule == EnvRuleError {
											return nil, errorAndLog("STR redefine ENV not found. para=%s, line=%d, file=%s", gx.Para, gx.Head, seg.File)
										} else {
											envx[rg.Para] = ""
											logDebug("checked STR redefine ENV, set Empty, Arg's line=%d, para=%s", gx.Head, gx.Para)
										}

									}
								} else { // REF
									if ex, kx := holdExe[gx.Para]; kx {
										holdExe[gx.Hold] = ex
										tx := &Arg{gx.Line, gx.Head, gx.Type, rg.Para, gx.Hold}
										argx[gx.Hold] = tx
										ex.Defs[gx.Hold] = rg.Para
										logDebug("appended Exe's STR redef REF, From=%d, To=%d, para=%s", gx.Head, rg.Head, rg.Para)
									} else {
										return nil, errorAndLog("STR redefine REF not found. para=%s, line=%d, file=%s", gx.Para, gx.Head, seg.File)
									}
								}
							}
						case CmndRun, CmndOut:
							exe.Acts = append(exe.Acts, gx)
							logDebug("appended Exe's %s, Arg's line=%d, hold=%s", gx.Type, gx.Head, gx.Hold)
						}
					}
				}
			}
			if len(defs) > 0 {
				exe.Defs = defs
			}
			iarg = -1
		}

		// 分析HOLD依赖
		var deps []*Hld // HOLD依赖
		stmt := exe.Seg.Text
		for hd, ag := range argx {
			for off, lln := 0, len(hd); true; off = off + lln {
				p := strings.Index(stmt[off:], hd)
				if p < 0 {
					break
				}

				off = off + p // 更新位置
				deps = append(deps, &Hld{off, off + lln, ag.Para, hd, !holdStr[hd]})
				// 引用计数
				holdCnt[hd] = holdCnt[hd] + 1

			}
		}
		// 必须有序
		sort.Slice(deps, func(i, j int) bool {
			return deps[i].Off < deps[j].Off
		})
		exe.Deps = deps

		// 挂树
		top := true
		for _, v := range exe.Acts {
			if pa, ok := holdExe[v.Hold]; ok {
				sonFunc(pa, exe, &top)
				logDebug("find %s parent, hold=%s, parent=%s, child=%s", v.Type, v.Hold, pa.Seg.Line, exe.Seg.Line)
				continue
			}

			if v.Para == ParaHas || v.Para == ParaNot {
				// not check
			} else {
				return nil, errorAndLog("%s HOLD's REF not found, hold=%s, line=%d, file=%s", v.Type, v.Hold, v.Head, seg.File)
			}
		}

		if top {
			for _, v := range exe.Deps {
				pa, ok := holdExe[v.Str]
				if ok { // REF|STR HOLD
					sonFunc(pa, exe, &top)
					logDebug("find DEP parent, hold=%s, parent=%s, child=%s", v.Str, pa.Seg.Line, exe.Seg.Line)
				}
			}
		}

		// 检查是否多库DEF
		if len(exe.Defs) > 0 {
			for _, v := range exe.Acts {
				if v.Type == CmndOut {
					return nil, errorAndLog("OUT used on Defs(REF,STR), seg=%#v", exe.Seg)
				}
			}
		}

		if top {
			tops = append(tops, exe)
		}

		alls = append(alls, exe)
		LogTrace("built an Exe, line=%s, file=%s", seg.Line, seg.File)
	}

	// 清理无用REF
	for hd, ct := range holdCnt {
		if ct > 0 {
			continue
		}

		if exe, ok := holdExe[hd]; ok {
			delete(exe.Defs, hd)
			LogTrace("remove unused REF|STR, arg=%#v", argx[hd])
		}
	}

	// 重排Sons，权重 `REF`<`ONE`<`FOR`<`END`，同级时算SQL位置。
	for i := 0; i < len(alls); i++ {

		rs := false
		if sons := alls[i].Sons; len(sons) > 0 {
			sort.Slice(sons, func(i, j int) bool {
				si, sj := sons[i], sons[j]
				wi, wj := -1, -1
				for _, v := range si.Acts {
					if w := weightArg(v); wi < w {
						wi = w
					}
				}
				for _, v := range sj.Acts {
					if w := weightArg(v); wj < w {
						wj = w
					}
				}

				ls := false
				if wi == wj {
					ls = si.Seg.Head < sj.Seg.Head
				} else {
					ls = wi < wj
				}
				if !ls {
					rs = true
				}

				return ls
			})
		}

		if rs {
			logDebug("resort Sons, line=%s", alls[i].Seg.Line)
		}
	}

	LogTrace("built a SQLX")

	sqlx := &SqlExe{envx, tops}
	return sqlx, nil
}

func (x Exe) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n{\nSql:%#v", x.Seg))

	if len(x.Defs) > 0 {
		sb.WriteString(" \nDefs:[")
		for h, p := range x.Defs {
			sb.WriteString(fmt.Sprintf("\n   hold:%s, para:%s", h, p))
		}
		sb.WriteString("]")
	}

	if len(x.Deps) > 0 {
		sb.WriteString(" \nDeps:[")
		for _, v := range x.Deps {
			sb.WriteString(fmt.Sprintf("\n   %#v", v))
		}
		sb.WriteString("]")
	}

	if len(x.Acts) > 0 {
		sb.WriteString(" \nActs:[")
		for i, v := range x.Acts {
			sb.WriteString(fmt.Sprintf("\n   %d:%#v", i, *v))
		}
		sb.WriteString("]")
	}

	if len(x.Sons) > 0 {
		sb.WriteString(" \nSons:[")
		for i, v := range x.Sons {
			son := fmt.Sprintf("%v", v)
			idx := fmt.Sprintf("{ // index=%d", i)
			son = strings.Replace(son, "{", idx, 1)
			son = strings.Replace(son, "\n", "\n   |    ", -1)
			sb.WriteString(son)
		}
		sb.WriteString("]")
	}
	sb.WriteString("\n}")
	return sb.String()
}

func (x *Exe) Tree() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nid=%d", x.Seg.Head))
	for _, v := range x.Sons {
		son := fmt.Sprintf("%s", v.Tree())
		son = strings.Replace(son, "\n", "\n  |  ", -1)
		son = strings.Replace(son, "  id", "--id", -1)
		sb.WriteString(son)
	}
	return sb.String()
}

func weightArg(p *Arg) int {
	switch p.Type {
	case CmndRun, CmndOut:
		for i, v := range paraWgt {
			if p.Para == v {
				return i
			}
		}

		return -1
	default:
		return -1
	}
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
				if cp := countQuotePair(sm[2]); cp > 0 { //脱去变量最外层引号
					sm[2] = sm[2][1 : len(sm[2])-1]
				}
			} else if cmd == CmndStr { // 相同才脱引号
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
