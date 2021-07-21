package art

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	//
	magicA9 = "${>a9+yeah+j7<ivy}"
	magicJ7 = "@"
	magicDs = magicA9 + magicJ7 + EnvSrcDb // need prefix
	magicDo = magicA9 + magicJ7 + EnvOutDb // need prefix

	HoldTop = "ITSELF"

	CmndEnv = "ENV"
	CmndVar = "VAR"
	CmndRef = "REF"
	CmndStr = "STR"
	CmndSeq = "SEQ"
	CmndTbl = "TBL"
	CmndRun = "RUN"
	CmndOut = "OUT"

	ParaOne = "ONE"
	ParaFor = "FOR"
	ParaEnd = "END"
	ParaHas = "HAS"
	ParaNot = "NOT"
)

var cmdArrs = []string{CmndEnv, CmndVar, CmndRef, CmndStr, CmndSeq, CmndTbl, CmndRun, CmndOut}

var argsReg = regexp.MustCompile(`(?i)` + // 不区分大小写
	`^[^0-9A-Z]*` + // 非英数开头，视为注释部分
	`(` + strings.Join(cmdArrs, "|") + `)[ \t]+` + //命令和空白，第一分组，固定值
	"([^`'\" \t]+|'[^']+'|\"[^\"]+\"|`+[^`]+`+)[ \t]+" + // 变量和空白，第二分组，
	"([^`'\" \t]+|'[^']+'|\"[^\"]+\"|`[^`]+`)") // 连续的非引号空白或，引号成对括起来的字符串（贪婪）

type SqlExe struct {
	Envs map[string]string // HOLD对应的环境变量
	Exes []*Exe            // 数据树
}

type Exe struct {
	Seg Sql // 对应的SQL片段

	// 循环
	Fors []*Arg // SEQ,TBL产生循环
	// 产出
	Refs map[string]string // REF|STR的提取，key=HOLD,val=PARA
	// 行为
	Acts []*Arg // 源库RUN & OUT
	// 依赖，顺序重要
	Deps []*Hld // 依赖
	// 运行，深度优先L
	Sons []*Exe // 分叉
}

type Arg struct {
	Line string      // 开始和结束行，全闭区间
	Head int         // 首行
	Type string      // 参数类型 Cmnd*
	Para string      // 变量名
	Hold string      // 占位符
	Gift interface{} // 附加对象（解析物，关联物）
}

type GiftSeq struct {
	Bgn int    // 开始
	End int    // 结束
	Inc int    // 步长
	Fmt string // 格式
}

type Hld struct {
	Off int    // 开始位置，包含
	End int    // 结束位置，包括
	Arg *Arg   // 对应的Arg
	Str string // HOLD字符串
	Dyn bool   // true：动态替换；false：静态替换
}

func ParseSqlx(sqls []Sql, envs map[string]string) (*SqlExe, error) {
	LogDebug("build a SQLX")

	// 除了静态环境变量，都是运行时确定的。
	holdExe := make(map[string]*Exe)   // hold出生的Exe
	holdCnt := make(map[string]int)    // HOLD引用计数
	holdStr := make(map[string]bool)   // HOLD为STR指令的
	lineArg := make(map[string][]*Arg) // 语句块和ARG

	var tops, alls []*Exe
	argx := make(map[string]*Arg)   // hold对应的Arg
	envx := make(map[string]string) // hold对应的ENV

	linkBase := func(base, cur *Exe) {
		not := true
		for i := len(base.Sons) - 1; i >= 0; i-- {
			if base.Sons[i].Seg.Head == cur.Seg.Head {
				not = false
				break
			}
		}
		if not {
			base.Sons = append(base.Sons, cur)
		}
	}

	iarg := -1
	sqlChk := make(map[int]int)
	for i, seg := range sqls {

		if seg.Exeb {
			sqlChk[seg.Head] = 0
		} else { // 解析指令
			if iarg < 0 {
				iarg = i
			}
			ags := parseArgs(seg.Text, seg.Head)
			for _, gx := range ags {
				lineArg[seg.Line] = append(lineArg[seg.Line], gx)
				// 定义指令，检测重复
				LogDebug("parsed Arg=%s", gx)

				if gx.Type == CmndRun || gx.Type == CmndOut {
					// 允许重复，不检查
				} else {
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

		LogDebug("build an Exe, line=%s, file=%s", seg.Line, seg.File)

		// 解析
		var fors []*Arg
		if iarg >= 0 { //有注释的
			refs := make(map[string]string)
			rule := envs[EnvRule]
			for j := iarg; j < i; j++ {
				if arg, ok := lineArg[sqls[j].Line]; ok {
					for _, gx := range arg {
						switch gx.Type {
						case CmndEnv:
							if ev, kx := envs[gx.Para]; kx {
								if gx.Para == EnvSrcDb {
									envx[gx.Hold] = magicDs
									LogDebug("checked runtime ENV, Arg's line=%d, para=%s, env=%s", gx.Head, gx.Para, magicDs)
								} else if gx.Para == EnvOutDb {
									envx[gx.Hold] = magicDo
									LogDebug("checked runtime ENV, Arg's line=%d, para=%s, env=%s", gx.Head, gx.Para, magicDo)
								} else {
									envx[gx.Hold] = ev
									LogDebug("checked def ENV, Arg's line=%d, para=%s, env=%s", gx.Head, gx.Para, ev)
								}
							} else {
								// 执行ENV
								if qc := countQuotePair(gx.Para); qc > 0 {
									envx[gx.Hold] = fmt.Sprintf("%s%s%d%s%s", magicA9, magicJ7, qc, magicJ7, gx.Para)
									LogDebug("got runtime ENV, Arg's line=%d, para=%s", gx.Head, gx.Para)
								} else {
									if rule == EnvRuleEmpty {
										envx[gx.Hold] = ""
										LogDebug("checked def ENV, set Empty, Arg's line=%d, para=%s", gx.Head, gx.Para)
									} else {
										return nil, errorAndLog("ENV not found. para=%s, line=%d, file=%s", gx.Para, gx.Head, seg.File)
									}
								}
							}
						case CmndSeq:
							// parse tx_test_%02d[1,20]
							p1 := strings.LastIndexByte(gx.Para, '[')
							p2 := strings.LastIndexByte(gx.Para, ']')
							if p2 <= p1 {
								return nil, errorAndLog("Bad format of SEQ. arg=%s, file=%s", gx, seg.File)
							}
							sp := strings.SplitN(gx.Para[p1+1:p2], ",", 3)
							ln := len(sp)
							if ln < 2 {
								return nil, errorAndLog("Bad format of SEQ, need [s,e,i]. arg=%s, file=%s", gx, seg.File)
							}
							it := make([]int, 3)
							for i, v := range sp {
								ip, er := strconv.ParseInt(strings.TrimSpace(v), 10, 32)
								if er != nil {
									return nil, errorAndLog("Bad format of SEQ, failed to parse int. arg=%s, file=%s, err=%v", gx, seg.File, er)
								}
								it[i] = int(ip)
							}
							if ln == 2 {
								it[2] = 1
							}

							gx.Gift = GiftSeq{it[0], it[1], it[2], strings.TrimSpace(gx.Para[0:p1])}

							holdExe[gx.Hold] = exe
							holdStr[gx.Hold] = true
							fors = append(fors, gx)
							LogDebug("appended Exe's SEQ, line=%d, para=%s, gift=%#v", gx.Head, gx.Para, gx.Gift)
						case CmndTbl:
							reg, er := regexp.Compile(gx.Para)
							if er != nil {
								return nil, errorAndLog("Bad format of TBL, failed to compile regexp. arg=%s, file=%s, err=%v", gx, seg.File, er)
							}
							gx.Gift = reg
							holdExe[gx.Hold] = exe
							holdStr[gx.Hold] = true
							fors = append(fors, gx)
							LogDebug("appended Exe's TBL, line=%d, para=%s", gx.Head, gx.Para)
						case CmndVar:
							refs[gx.Hold] = gx.Para
							LogDebug("appended Exe's VAR, Arg's line=%d, para=%s, hold=%s", gx.Head, gx.Para, gx.Hold)
						case CmndRef:
							refs[gx.Hold] = gx.Para
							holdExe[gx.Hold] = exe
							LogDebug("appended Exe's REF, Arg's line=%d, para=%s, hold=%s", gx.Head, gx.Para, gx.Hold)
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
									if gx.Para == EnvSrcDb {
										envx[gx.Hold] = magicDs
										LogDebug("checked runtime STR def ENV, Arg's line=%d, para=%s, env=%s", gx.Head, gx.Para, magicDs)
									} else if gx.Para == EnvOutDb {
										envx[gx.Hold] = magicDo
										LogDebug("checked runtime STR def ENV, Arg's line=%d, para=%s, env=%s", gx.Head, gx.Para, magicDo)
									} else {
										envx[gx.Hold] = ev
										LogDebug("checked STR def ENV, Arg's line=%d, para=%s, env=%s", gx.Head, gx.Para, ev)
									}
								} else { // REF
									holdExe[gx.Hold] = exe
									refs[gx.Hold] = gx.Para
									LogDebug("appended Exe's STR def REF, Arg's line=%d, hold=%s", gx.Head, gx.Hold)
								}
							} else { // 重新定义
								if rg.Type == CmndEnv { // 重定义的ENV
									if ev, kx := envs[rg.Para]; kx {
										envx[gx.Hold] = ev
										LogDebug("checked STR redefine ENV, Arg's line=%d, para=%s, env=%s", gx.Head, rg.Para, ev)
									} else {
										if qc := countQuotePair(rg.Para); qc > 0 {
											envx[gx.Hold] = envx[rg.Hold]
											LogDebug("checked STR redefine runtime ENV, Arg's line=%d, para=%s", gx.Head, gx.Para)
										} else {
											if rule == EnvRuleEmpty {
												envx[gx.Hold] = ""
												LogDebug("checked STR redefine ENV, set Empty, Arg's line=%d, para=%s", gx.Head, gx.Para)
											} else {
												return nil, errorAndLog("STR redefine ENV not found. para=%s, line=%d, file=%s", gx.Para, gx.Head, seg.File)
											}
										}
									}
								} else if rg.Type == CmndRef { // REF
									if ex, kx := holdExe[gx.Para]; kx {
										holdExe[gx.Hold] = ex
										argx[gx.Hold] = &Arg{gx.Line, gx.Head, gx.Type, rg.Para, gx.Hold, rg}
										if ex.Refs == nil {
											return nil, errorAndLog("never go here, From=%d, To=%d, para=%s", gx.Head, rg.Head, rg.Para)
										}
										ex.Refs[gx.Hold] = rg.Para
										LogDebug("appended Exe's STR redef REF, From=%d, To=%d, para=%s", gx.Head, rg.Head, rg.Para)
									} else {
										return nil, errorAndLog("STR redefine REF not found. para=%s, line=%d, file=%s", gx.Para, gx.Head, seg.File)
									}
								} else {
									return nil, errorAndLog("unsupported STR redefine. para=%s, line=%d, file=%s", gx.Para, gx.Head, seg.File)
								}
							}
						case CmndRun, CmndOut:
							exe.Acts = append(exe.Acts, gx)
							LogDebug("appended Exe's %s, Arg's line=%d, hold=%s", gx.Type, gx.Head, gx.Hold)
						}
					}
				}
			}
			if len(refs) > 0 {
				exe.Refs = refs
			}
			iarg = -1
		}
		exe.Fors = fors

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
				deps = append(deps, &Hld{off, off + lln, ag, hd, !holdStr[hd]})
				// 引用计数
				holdCnt[hd] = holdCnt[hd] + 1

			}
		}
		// 必须有序
		sort.Slice(deps, func(i, j int) bool {
			return deps[i].Off < deps[j].Off
		})
		exe.Deps = deps

		// 挂树， 寻到一个即可
		// 优先级 RUN|OUT > REF > SEQ|TBL
		isTop := true
		// 优先级 10，支持多父，RUN|OUT
		for _, v := range exe.Acts {
			if pa, ok := holdExe[v.Hold]; ok {
				linkBase(pa, exe)
				LogDebug("bind %s parent, hold=%s, parent=%s, child=%s", v.Type, v.Hold, pa.Seg.Line, exe.Seg.Line)
				isTop = false
				continue
			}

			if v.Para == ParaHas || v.Para == ParaNot {
				LogDebug("skip parent, %s. line=%s", v.Type, exe.Seg.Line)
			} else if v.Hold == HoldTop {
				LogDebug("bind parent to ITSELF , %s. line=%s", v.Type, exe.Seg.Line)
			} else {
				return nil, errorAndLog("%s HOLD's REF not found, hold=%s, line=%d, file=%s", v.Type, v.Hold, v.Head, seg.File)
			}
		}

		// 优先级 单父, 20 REF|STR, 30 SEQ|TBL
		if isTop {
			var b20, b30 *Exe
			var h20, h30 string
			for _, v := range exe.Deps {
				if pa, ok := holdExe[v.Str]; ok && exe.Seg.Head != pa.Seg.Head {
					typ := v.Arg.Type
					if (typ == CmndStr || typ == CmndRef) && (b20 == nil || b20.Seg.Head < pa.Seg.Head) {
						b20 = pa
						h20 = v.Str
						continue
					}
					if (typ == CmndSeq || typ == CmndTbl) && (b30 == nil || b30.Seg.Head < pa.Seg.Head) {
						b30 = pa
						h30 = v.Str
						continue
					}
				}
			}

			bs, hs := b30, h30
			if b20 != nil {
				bs, hs = b20, h20
			}

			if bs != nil {
				linkBase(bs, exe)
				LogDebug("bind DEP parent, parent=%s, child=%s, by hold=%s", bs.Seg.Line, exe.Seg.Line, hs)
				isTop = false
			}
		}

		// post check current
		// 检查是否多库 OUT Def
		if len(exe.Refs) > 0 || len(exe.Fors) > 0 {
			for _, v := range exe.Acts {
				if v.Type == CmndOut {
					return nil, errorAndLog("OUT used on Defs(REF,STR), seg=%#v", exe.Seg)
				}
			}
		}

		if isTop {
			tops = append(tops, exe)
			LogDebug("append top exe head=%d", exe.Seg.Head)
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
			delete(exe.Refs, hd)
			LogTrace("remove unused REF|STR, arg=%#v", argx[hd])
		}
	}

	for i := 0; i < len(alls); i++ {
		if sons := alls[i].Sons; len(sons) > 0 {
			sort.Slice(sons, func(i, j int) bool {
				si, sj := sons[i], sons[j]
				return si.Seg.Head < sj.Seg.Head
			})
		}
	}

	// post check
	checkExe(tops, sqlChk)
	postBad := false
	for k, v := range sqlChk {
		if v != 1 {
			postBad = true
			if v < 0 {
				LogError("exe from nil sql, line=%d", k)
			} else {
				LogError("sql to more exe, line=%d", k)
			}
		}
	}

	if postBad {
		return nil, errorAndLog("failed to post check")
	}

	LogTrace("built a SQLX")
	sqlx := &SqlExe{envx, tops}
	return sqlx, nil
}

func (h *Hld) String() string {
	return fmt.Sprintf("Hld{Off:%d, End:%d, Arg.head:%d, Dyn:%t, Str:%q}",
		h.Off,
		h.End,
		h.Arg.Head,
		h.Dyn,
		h.Str,
	)
}

func (h *Arg) String() string {
	return fmt.Sprintf("Arg{Head:%d, Type:%s, Para:%s, Hold:%s, Gift-nil:%t}",
		h.Head,
		h.Type,
		h.Para,
		h.Hold,
		h.Gift == nil,
	)
}

func (x *Exe) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n{\nSql:%#v", x.Seg))

	if len(x.Fors) > 0 {
		sb.WriteString(" \nFors:[")
		for h, p := range x.Fors {
			sb.WriteString(fmt.Sprintf("\n   hold:%d, arg:%s", h, p))
		}
		sb.WriteString("]")
	}

	if len(x.Refs) > 0 {
		sb.WriteString(" \nRefs:[")
		for h, p := range x.Refs {
			sb.WriteString(fmt.Sprintf("\n   hold:%s, para:%s", h, p))
		}
		sb.WriteString("]")
	}

	if len(x.Deps) > 0 {
		sb.WriteString(" \nDeps:[")
		for _, v := range x.Deps {
			sb.WriteString(fmt.Sprintf("\n   %s", v))
		}
		sb.WriteString("]")
	}

	if len(x.Acts) > 0 {
		sb.WriteString(" \nActs:[")
		for i, v := range x.Acts {
			sb.WriteString(fmt.Sprintf("\n   %d:%s", i, v))
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

func checkExe(exes []*Exe, sqls map[int]int) {
	for _, v := range exes {
		if c, ok := sqls[v.Seg.Head]; ok {
			sqls[v.Seg.Head] = c + 1
		} else {
			sqls[v.Seg.Head] = -1
		}
		checkExe(v.Sons, sqls)
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
			} else if cmd == CmndVar || cmd == CmndRef || cmd == CmndEnv {
				//脱去变量最外层引号，SQL会保留至少一个反单引号
				if cp := countQuotePair(sm[2]); cp > 0 {
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

			arg := &Arg{ln, i + h, cmd, sm[2], sm[3], nil}
			args = append(args, arg)
		}
	}
	return
}
