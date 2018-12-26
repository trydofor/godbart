package art

import (
	"bytes"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func Tree(pref *Preference, envs map[string]string, srce *DataSource, dest []*DataSource, file []FileEntity, risk bool) error {

	sqlx, err := ParseTree(pref, envs, file)
	if err != nil {
		return err
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
		CtrlRoom.putEnv(roomTreeEnvSqlx, exe)
		er := RunSqlx(pref, exe, scon, dcon, risk)
		if er != nil {
			return er
		}
	}

	return nil
}

func ParseTree(pref *Preference, envs map[string]string, file []FileEntity) ([]*SqlExe, error) {
	sqlx := make([]*SqlExe, 0, len(file))
	for _, f := range file {
		sqls := ParseSqls(pref, &f)
		exe, er := ParseSqlx(sqls, envs)
		if er != nil {
			LogError("failed to parse sqlx, file=%s", f.Path)
			return nil, er
		}
		sqlx = append(sqlx, exe)
	}
	return sqlx, nil
}

func RunSqlx(pref *Preference, sqlx *SqlExe, src *MyConn, dst []*MyConn, risk bool) error {

	ctx := make(map[string]interface{}) // 存放select的REF

	for k, v := range sqlx.Envs {
		ctx[k] = v
	}

	ncm, lcm, dlt := "\n"+pref.LineComment+" ", pref.LineComment, pref.DelimiterRaw
	var outf = func(exe *Exe, sql string, src bool) {

		rsrc, rout := takeSrcOutAct(exe)
		if src {
			if !rsrc {
				return
			}
		} else {
			if !rout {
				return
			}
		}

		one, end, ech := actOneEndFor(exe)
		buf := bytes.NewBuffer(make([]byte, 0, 50))
		buf.WriteString(fmt.Sprintf("ID=%d, LINE=%s", exe.Seg.Head, exe.Seg.Line))

		if ech {
			buf.WriteString(", FOR")
		}
		if end {
			buf.WriteString(", END")
		}
		if one {
			buf.WriteString(", ONE")
		}

		if len(exe.Deps) > 0 && !ech && !end && !one {
			buf.WriteString(", DEP")
		}

		info := buf.String()
		if src {
			OutTrace("%s%s SRC %s", ncm, lcm, info)
			OutDebug("%s%s", sql, dlt)
		} else {
			ddq := strings.Replace(sql, "\n", ncm, -1)
			OutTrace("%s%s OUT %s", ncm, lcm, info)
			OutDebug("%s %s%s", lcm, ddq, dlt)
		}
	}

	for _, exe := range sqlx.Exes {
		er := runExe(exe, src, dst, ctx, outf, risk, 1)
		if er != nil {
			return er
		}
	}

	return nil
}

var defValCol = regexp.MustCompile(`(VAL|COL)\[([^\[\]]*)\]`)

func runExe(exe *Exe, src *MyConn, dst []*MyConn, ctx map[string]interface{}, outf func(exe *Exe, str string, src bool), risk bool, lvl int) error {

	// 判断数据源和执行条件
	if arg, igr := skipHasNotRun(src, exe.Acts, ctx); igr {
		LogTrace("SKIP exe on Condition. arg=%d seg=%d", arg.Head, exe.Seg.Head)
		logDebug("arg=%#v seg=%#v", arg, exe.Seg)
		return nil
	}

	// 构造执行语句
	stmt, prnt, vals, err := buildStatement(exe, ctx, src)
	if err != nil {
		return err
	}

	// \n-- 前缀

	outf(exe, prnt, true)
	outf(exe, prnt, false)

	head := exe.Seg.Head
	line := exe.Seg.Line

	jobx := true // 保证执行
	defer func() {
		if jobx {
			CtrlRoom.dealJobx(nil, head)
		}
	}()

	LogTrace("take stmt, id=%d, lvl=%d line=%s", head, lvl, line)
	logDebug("stmt= %q", stmt)
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
				jobx = true
				LogTrace("deal %d-th row, id=%d, line=%s", cnt, head, line)
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
									logDebug("simple sys DEF, hold=%s, para=%s, col-name=%s", hld, ptn, cln)
								} else { // VAL
									ctx[hld] = vals[j]
									dbt := cols[j].DatabaseTypeName()
									ctx[hld+":DatabaseTypeName"] = dbt
									logDebug("simple sys DEF, hold=%s, para=%s, value=%#v, dbtype=%s", hld, ptn, vals[j], dbt)
								}
							} else {
								pld := fmt.Sprintf("%s:%d", hld, k) // 保证多值的不能直接找到
								if sub[1] == "COL" {
									cls := make([]string, ln)
									for i, c := range cols {
										cls[i] = c.Name()
									}
									ctx[pld] = cls
									logDebug("simple sys DEF, hold=%s, para=%s, values'count=%d", pld, ptn, len(cls))
								} else {
									dbt := make([]string, ln)
									for i, c := range cols {
										dbt[i] = c.DatabaseTypeName()
									}
									ctx[pld] = vals
									ctx[pld+":DatabaseTypeName"] = dbt
									logDebug("simple sys DEF, hold=%s, para=%s, value'count=%d", pld, ptn, len(dbt))
								}
							}
						}
					}

					for i := 0; lost && i < ln; i++ {
						if strings.EqualFold(cols[i].Name(), ptn) {
							ctx[hld] = vals[i]
							dbt := cols[i].DatabaseTypeName()
							ctx[hld+":DatabaseTypeName"] = dbt
							ltr, _ := src.Literal(vals[i], dbt)
							logDebug("simple usr DEF, hold=%s, para=%s, value=%s", hld, ptn, ltr)
							lost = false
							break
						}
					}

					if lost {
						return errorAndLog("failed to resolve DEF. hold=%s, para=%s, in seg=%#v", hld, ptn, exe.Seg)
					}
				}

				// 遍历子树, ONE,FOR,END
				for _, son := range exe.Sons {
					if !shouldForAct(son, cnt) {
						continue
					}
					LogTrace("fork ONE/FOR child=%d, parent=%d", son.Seg.Head, head)
					er := runExe(son, src, dst, ctx, outf, risk, lvl+1)
					if er != nil {
						return er
					}
				}

				// 每个记录一棵树
				jobx = false
				CtrlRoom.dealJobx(nil, head)
			}

			// 有记录时，遍历END子树
			if cnt > 0 {
				for _, son := range exe.Sons {
					if !shouldEndAct(son, cnt) {
						continue
					}
					LogTrace("fork END child=%d, parent=%d", son.Seg.Head, head)
					er := runExe(son, src, dst, ctx, outf, risk,lvl+1)
					if er != nil {
						return er
					}
				}
			}

			LogTrace("done %d rows, id=%d, line=%s", cnt, head, line)

			return nil
		}
		//
		er := src.Query(ff, stmt, vals...)
		if er != nil {
			return er
		}
	} else {
		rsrc, rout := takeSrcOutAct(exe)
		dcnt := len(dst)
		if risk {
			if rsrc {
				LogTrace("running on SRC db=%s", src.DbName())
				if a, e := src.Exec(stmt, vals...); e != nil {
					LogError("failed on SRC db=%s, err=%v", src.DbName(), e)
					return e
				} else {
					LogTrace("done %d affected on SRC db=%s", a, src.DbName())
				}
			}

			if rout {
				for i, db := range dst {
					LogTrace("running on OUT[%d/%d] db=%s", i+1, dcnt, db.DbName())
					if a, e := db.Exec(stmt, vals...); e != nil {
						LogError("failed on OUT[%d/%d] db=%s, err=%v", i+1, dcnt, db.DbName(), e)
						return e
					} else {
						LogTrace("done %d affected on OUT[%d/%d] db=%s", a, i+1, dcnt, db.DbName())
					}
				}
			}
		} else {
			if rsrc {
				LogTrace("fake run on SRC db=%s", src.DbName())
			}

			if rout {
				for i, db := range dst {
					LogTrace("fake run on OUT[%d/%d] db=%s", i+1, dcnt, db.DbName())
				}
			}
		}
	}

	LogTrace("done stmt, id=%d, lvl=%d, line=%s\n", head, lvl, line)
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

func takeSrcOutAct(exe *Exe) (bool, bool) {

	src, out := false, false
	for _, v := range exe.Acts {
		if v.Type == CmndOut {
			out = true
		} else if v.Type == CmndRun {
			src = true
		}
	}

	if out {
		// 有OUT时，必须有RUN才在SRC上执行
	} else {
		// 没OUT时，默认在SRC上执行
		src = true
	}

	return src, out
}

func actOneEndFor(exe *Exe) (one, end, ech bool) {
	one, end, ech = false, false, false
	for _, arg := range exe.Acts {
		switch arg.Para {
		case ParaOne:
			one = true
		case ParaFor:
			ech = true
		case ParaEnd:
			end = true
		}
	}

	return
}

func shouldForAct(exe *Exe, cnt int) bool {

	one, end, ech := actOneEndFor(exe)

	// 有END时，必须有FOR
	if end {
		return ech
	}

	// 有ONE时,执行对一个
	if one {
		if ech {
			return true
		} else {
			return cnt == 1
		}
	}

	// 默认是FOR
	return true
}

func shouldEndAct(exe *Exe, cnt int) bool {

	one, end, ech := actOneEndFor(exe)

	// 有FOR的时候，END会在FOR中执行
	if ech {
		return false
	}

	// 只有一条记录，且被ONE执行过了
	if one && cnt == 1 {
		return false
	}

	return end
}

func buildStatement(exe *Exe, ctx map[string]interface{}, src *MyConn) (stmt, prnt string, vals []interface{}, err error) {
	stmt = exe.Seg.Text
	prnt = stmt

	if hlen := len(exe.Deps); hlen > 0 {
		logDebug("building line=%s,stmt=%#v", exe.Seg.Line, stmt)
		vals = make([]interface{}, 0, hlen)
		var rtn, std strings.Builder // return,stdout
		off := 0
		for _, dep := range exe.Deps {
			logDebug("parsing dep=%#v", dep)

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
					logDebug("dynamic replace hold=%s, with quote=%t, value=%s", hld, b, v)
				} else {
					rtn.WriteString(v)
					std.WriteString(v)
					logDebug("static simple replace hold=%s, with value=%s", hld, v)
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
					err = errorAndLog("can not find hold, check the REF has record. hold=%s, para=%s", hld, ptn)
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
							logDebug("use joiner=%s. hold=%s, index=%d", jner, hld, k)
						} else if k == mtln-1 {
							jner = ","
							logDebug("user default joiner=%s", jner)
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
						mval = append(mval, hv, EmptyArr)
						logDebug("get %d COL values. hold=%s, para=%s", k, hld, ptn)
					} else {
						dv, dk := ctx[pld+":DatabaseTypeName"]
						if !dk {
							err = errorAndLog("failed to get %d hold's type. hold=%s, para=%s", k, hld, ptn)
							return
						}
						mval = append(mval, dv, hv)
						logDebug("get %d VAL values. hold=%s, para=%s", len(dv.([]string)), hld, ptn)
					}
				}

				// 处理数据
				logDebug("deal pattern STR with %d items", itct)
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
		prnt = std.String()
	}

	return
}
