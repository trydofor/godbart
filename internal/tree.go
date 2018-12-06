package internal

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

const (
	ParaFor = "FOR"
	ParaEnd = "END"
	ParaHas = "HAS"
	ParaNot = "NOT"
)

func Tree(pref *Preference, envs map[string]string, srce *DataSource, dest []*DataSource, file []FileEntity, risk bool) error {

	sqlx := make([]*SqlExe, 0, len(file))
	for _, f := range file {
		sqls := ParseSqls(pref, &f)
		exe, er := ParseSqlx(sqls, envs)
		if er != nil {
			log.Fatalf("[ERROR] failed to parse sqlx, file=%s\n", f.Path)
			return er
		}
		sqlx = append(sqlx, exe)
	}

	scon, err := openDbAndLog(srce)
	if err != nil {
		return err
	}

	dcon := make([]*MyConn, 0, len(dest))
	for _, v := range dest {
		conn, er := openDbAndLog(v)
		if er != nil {
			return er
		}
		dcon = append(dcon, conn)
	}

	for _, exe := range sqlx {
		er := exe.Run(scon, dcon, risk)
		if er != nil {
			return er
		}
	}

	return nil
}

func (sqlx *SqlExe) Run(src *MyConn, dst []*MyConn, test bool) error {

	ctx := make(map[string]interface{}) // 存放select的REF

	for k, v := range sqlx.Envs {
		ctx[k] = v
	}

	for _, exe := range sqlx.Exes {
		er := runExe(exe, src, dst, ctx, test)
		if er != nil {
			return er
		}
	}
	return nil
}

var defValCol = regexp.MustCompile(`(VAL|COL)\[([^\[\]]*)\]`)

func runExe(exe *Exe, src *MyConn, dst []*MyConn, ctx map[string]interface{}, risk bool) error {

	// 判断数据源和执行条件
	if arg, igr := skipHasNotRun(src, exe.Runs, ctx); igr {
		log.Printf("[TRACE] INGORE exe by RUN. arg=%#v seg=%#v\n", arg, exe.Seg)
		return nil
	}
	if arg, igr := skipHasNotRun(src, exe.Outs, ctx); igr {
		log.Printf("[TRACE] INGORE exe by OUT. arg=%#v seg=%#v\n", arg, exe.Seg)
		return nil
	}

	// 构造执行语句
	stmt, vals, err := buildStatement(exe, ctx, src)
	if err != nil {
		return err
	}

	log.Printf("[TRACE] plan stmt, line=%s, stmt=%s\n", exe.Seg.Line, stmt)

	if len(exe.Defs) > 0 { // 有结果集提取，不支持OUT
		var ff = func(row *sql.Rows) error {
			cols, er := row.ColumnTypes()
			if er != nil {
				return er
			}

			ln := len(cols)
			vals := make([]interface{}, ln)
			ptrs := make([]interface{}, ln)
			for i := 0; i < ln; i++ {
				ptrs[i] = &vals[i]
			}

			cnt := 0
			for row.Next() {
				cnt++
				log.Printf("[TRACE] processing %d rows, stmt=%s\n", cnt, stmt)
				row.Scan(ptrs...)

				//// 提取结果集
				for hld, ptn := range exe.Defs {
					lost := true
					if strings.Contains(ptn, "COL[") || strings.Contains(ptn, "VAL[") {
						// 内置模式
						mts := defValCol.FindAllStringSubmatch(ptn, -1)
						for k, sub := range mts {
							lost = false
							if j, ok := strconv.ParseInt(sub[2], 10, 32); ok == nil {
								j-- // 从1开始
								if sub[1] == "COL" {
									cln := cols[j].Name()
									ctx[hld] = cln
									log.Printf("[TRACE]     simple sys DEF, hold=%s, para=%s, col-name=%s\n", hld, ptn, cln)
								} else { // VAL
									ctx[hld] = vals[j]
									dbt := cols[j].DatabaseTypeName()
									ctx[hld+":DatabaseTypeName"] = dbt
									log.Printf("[TRACE]     simple sys DEF, hold=%s, para=%s, value=%#v, dbtype=%s\n", hld, ptn, vals[j], dbt)
								}
							} else {
								pld := fmt.Sprintf("%s:%d", hld, k) // 保证多值的不能直接找到
								if sub[1] == "COL" {
									cls := make([]string, ln)
									for i, c := range cols {
										cls[i] = c.Name()
									}
									ctx[pld] = cls
									log.Printf("[TRACE]     simple sys DEF, hold=%s, para=%s, col-names=%s\n", pld, ptn, strings.Join(cls, ","))
								} else {
									dbt := make([]string, ln)
									for i, c := range cols {
										dbt[i] = c.DatabaseTypeName()
									}
									ctx[pld] = vals
									ctx[pld+":DatabaseTypeName"] = dbt
									log.Printf("[TRACE]     simple sys DEF, hold=%s, para=%s, dbtypes=%s\n", pld, ptn, strings.Join(dbt, ","))

								}
							}
						}
					}

					for i := 0; lost && i < ln; i++ {
						if strings.EqualFold(cols[i].Name(), ptn) {
							ctx[hld] = vals[i]
							dbt := cols[i].DatabaseTypeName()
							ctx[hld+":DatabaseTypeName"] = dbt
							log.Printf("[TRACE]     simple usr DEF, hold=%s, para=%s, value=%#v, dbtype=%s\n", hld, ptn, vals[i], dbt)
							lost = false
							break
						}
					}

					if lost {
						return errorAndLog("failed to resolve DEF. hold=%s, para=%s, in seg=%#v", hld, ptn, exe.Seg)
					} else {

					}
				}

				// 遍历FOR子树
				for _, son := range exe.Sons {
					if !shouldForRun(son) {
						continue
					}
					log.Printf("[TRACE] fork FOR child, line=%s, parent=%s\n", son.Seg.Line, exe.Seg.Line)
					er := runExe(son, src, dst, ctx, risk)
					if er != nil {
						return er
					}
				}
			}
			// 遍历END子树
			for _, son := range exe.Sons {
				if !shouldEndRun(son) {
					continue
				}
				log.Printf("[TRACE] fork END child=%s, parent=%s\n", son.Seg.Line, exe.Seg.Line)
				er := runExe(son, src, dst, ctx, risk)
				if er != nil {
					return er
				}
			}

			log.Printf("[TRACE] processed %d rows, stmt=%s\n", cnt, stmt)

			return nil
		}
		//
		er := src.Query(ff, stmt, vals...)
		if er != nil {
			return er
		}
	} else {
		rsrc, rout := takeSrcOutRun(exe)

		if risk {
			if rsrc {
				log.Printf("[TRACE] running on SRC db=%s\n", src.DbName())
				if a, e := src.Exec(stmt, vals...); e != nil {
					log.Fatalf("[ERROR] failed on SRC db=%s, err=%v\n", src.DbName(), e)
					return e
				} else {
					log.Printf("[TRACE] done %d affected on SRC db=%s\n", a, src.DbName())
				}
			}

			if rout {
				for i, db := range dst {
					log.Printf("[TRACE] running on OUT[%d] db=%s\n", i, db.DbName())
					if a, e := db.Exec(stmt, vals...); e != nil {
						log.Fatalf("[ERROR] failed on OUT[%d] db=%s, err=%v\n", i, db.DbName(), e)
						return e
					} else {
						log.Printf("[TRACE] done %d affected on OUT[%d] db=%s\n", i, a, db.DbName())
					}
				}
			}
		} else {
			if rsrc {
				log.Printf("[TRACE] fake run on SRC db=%s\n", src.DbName())
			}

			if rout {
				for i, db := range dst {
					log.Printf("[TRACE] fake run on OUT[%d] db=%s\n", i, db.DbName())
				}
			}
		}
	}
	log.Printf("[TRACE] done stmt, line=%s\n\n", exe.Seg.Line)
	return nil
}

func skipHasNotRun(con *MyConn, args []*Arg, ctx map[string]interface{}) (*Arg, bool) {

	for _, arg := range args {
		if arg.Para == ParaHas || arg.Para == ParaNot {
			va := ctx[arg.Hold]
			no := con.Nothing(va)
			//
			if arg.Para == ParaHas && no {
				return arg, true
			}
			if arg.Para == ParaNot && !no {
				return arg, true
			}
		}
	}
	return nil, false
}

func takeSrcOutRun(exe *Exe) (bool, bool) {
	// 没OUT时，默认在SRC上执行
	// 有OUT时，必须有RUN才在SRC上执行

	src, out := true, true
	if len(exe.Outs) == 0 {
		out = false
	} else {
		src = len(exe.Runs) > 0
	}

	return src, out
}

func shouldForRun(exe *Exe) bool {
	// 有END时，必须有FOR
	// 默认是FOR
	if shouldEndRun(exe) {
		for _, arg := range exe.Runs {
			if arg.Para == ParaFor {
				return true
			}
		}
		for _, arg := range exe.Outs {
			if arg.Para == ParaFor {
				return true
			}
		}

		return false
	} else {
		return true
	}
}

func shouldEndRun(exe *Exe) bool {
	for _, arg := range exe.Runs {
		if arg.Para == ParaEnd {
			return true
		}
	}
	for _, arg := range exe.Outs {
		if arg.Para == ParaEnd {
			return true
		}
	}
	return false
}

var nxl = []interface{}{}

func buildStatement(exe *Exe, ctx map[string]interface{}, src *MyConn) (stmt string, vals []interface{}, err error) {
	stmt = exe.Seg.Text
	if hlen := len(exe.Deps); hlen == 0 {
		// 输出可执行的SQL
		fmt.Printf("\n-- %s\n%s\n", src.DbName(), stmt)
		if len(exe.Outs) > 0 { // 标记和注释，只有目标库SQL
			fmt.Printf("\n%s\n", strings.Replace(stmt, "\n", "\n-- ", -1))
		}
	} else {
		log.Printf("[TRACE] building statement=%s\n", stmt)
		vals = make([]interface{}, 0, hlen)
		var rtn, std strings.Builder // return,stdout
		off := 0
		for _, dep := range exe.Deps {
			log.Printf("[TRACE]   parsing dep=%#v\n", dep)

			if dep.Off > off {
				tmp := stmt[off:dep.Off]
				rtn.WriteString(tmp)
				std.WriteString(tmp)
			}

			off = dep.End
			hld, ptn := dep.Str, dep.Ptn

			if ev, ok := ctx[hld]; ok {
				dbt := ""
				if dpv, ok := ctx[hld+":DatabaseTypeName"]; ok {
					dbt = dpv.(string)
				}

				v, b := src.Literal(ev, dbt)

				if dep.Dyn { // 动态
					vals = append(vals, ev)
					rtn.WriteString("?")

					if b {
						std.WriteString(src.Quotesc(v, "'"))
					} else {
						std.WriteString(v)
					}
					log.Printf("[TRACE]     dynamic replace hold=%s, with quote=%t, value=%s\n", hld, b, v)
				} else {
					rtn.WriteString(v)
					std.WriteString(v)
					log.Printf("[TRACE]     static simple replace hold=%s, with value=%s\n", hld, v)
				}
			} else {
				// 多值或模式
				//espace
				var sb strings.Builder
				pln := len(ptn)
				for i := 0; i < pln; i++ {
					c := ptn[i]
					if c == '\\' && i < pln-1 {
						switch ptn[i+1] {
						case '\\':
							sb.WriteByte(c)
							i++
						case 'n':
							sb.WriteByte('\n')
							i++
						case 't':
							sb.WriteByte('\t')
							i++
						default:
							sb.WriteByte(c)
						}
					} else {
						sb.WriteByte(c)
					}
				}
				ptn = sb.String()
				pln = len(ptn)

				mts := defValCol.FindAllStringSubmatchIndex(ptn, -1)
				if len(mts) == 0 {
					err = errorAndLog("bad multiple hold. hold=%s, para=%s", hld, ptn)
					return
				}

				mtln := len(mts) // 模式数量
				jner := ""       // 分隔符
				mpos := make([]int, 0, mtln*2)
				mval := make([]interface{}, 0, mtln*2)
				itct := 0

				// 处理模板
				for k, sub := range mts {
					if len(jner) == 0 {
						if spt := ptn[sub[4]:sub[5]]; len(spt) > 0 {
							jner = spt
							log.Printf("[TRACE] use joiner=%s. hold=%s, index=%d\n", jner, hld, k)
						} else if k == mtln-1 {
							jner = ","
							log.Printf("[TRACE] user default joiner=%s\n", jner)
						}
					}

					mpos = append(mpos, sub[0], sub[1])

					pld := fmt.Sprintf("%s:%d", hld, k) // 保证多值的不能直接找到
					hv, ok := ctx[pld]
					if !ok {
						err = errorAndLog("failed to get %d hold's value. hold=%s, para=%s", k, hld, ptn)
						return
					}

					ct := 0
					switch xx := hv.(type) {
					case []string:
						ct = len(xx)
					case []interface{}:
						ct = len(xx)
					}

					if itct == 0 {
						itct = ct
					} else {
						if itct != ct {
							err = errorAndLog("pattern STR's item's length NOT equals, %d hold's value. hold=%s, para=%s", k, hld, ptn)
							return
						}
					}

					if ptn[sub[2]:sub[3]] == "COL" {
						mval = append(mval, hv, nxl)
						log.Printf("[TRACE]  get %d COL values. hold=%s, para=%s", k, hld, ptn)
					} else {
						dv, dk := ctx[pld+":DatabaseTypeName"]
						if !dk {
							err = errorAndLog("failed to get %d hold's type. hold=%s, para=%s", k, hld, ptn)
							return
						}
						mval = append(mval, dv, hv)
						log.Printf("[TRACE]  get %d VAL values. hold=%s, para=%s", k, hld, ptn)
					}
				}

				// 处理数据
				log.Printf("[TRACE] processing pattern STR with %d iterms", itct)
				for k := 0; k < itct; k++ {
					if k > 0 {
						rtn.WriteString(jner)
						std.WriteString(jner)
					}

					off := 0
					for m := 0; m < mtln; m++ {
						b, g := m*2, m*2+1
						if mpos[b] > off {
							tmp := ptn[off:mpos[b]]
							rtn.WriteString(tmp)
							std.WriteString(tmp)
						}
						vvv := mval[b].([]string)
						ttt := mval[g].([]interface{})

						if len(ttt) == 0 { // COL
							rtn.WriteString(vvv[k])
							std.WriteString(vvv[k])
						} else {
							vals = append(vals, ttt[k])
							rtn.WriteString("?")
							if str, qto := src.Literal(ttt[k], vvv[k]); qto {
								std.WriteString(src.Quotesc(str, "'"))
							} else {
								std.WriteString(str)
							}
						}

						off = mpos[g]

						if m == mtln-1 && off < pln {
							tmp := ptn[off:]
							rtn.WriteString(tmp)
							std.WriteString(tmp)
						}
					}
				}
			}
		}
		//
		if off < len(stmt) {
			pt2 := stmt[off:]
			rtn.WriteString(pt2)
			std.WriteString(pt2)
		}

		stmt = rtn.String()
		prnt := std.String()

		rsrc, rout := takeSrcOutRun(exe)

		if rsrc {
			fmt.Printf("\n-- %s\n%s\n", src.DbName(), prnt)
		}
		if rout {
			fmt.Printf("\n-- OUT\n-- %s\n", strings.Replace(prnt, "\n", "\n-- ", -1))
		}
	}

	return
}
