package art

import (
	"bytes"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
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
			LogFatal("failed to parse sqlx, file=%s", f.Path)
			return nil, er
		}
		sqlx = append(sqlx, exe)
	}
	return sqlx, nil
}

func stmtEnv(v string, src *MyConn, tmp map[string]string) (rst string, has bool, err error) {
	if !strings.HasPrefix(v, magicA9) {
		return
	}

	if sv, ho := tmp[v]; ho {
		return sv, true, nil
	}

	ptn := strings.SplitN(v, magicJ7, 3)
	if len(ptn) == 3 && ptn[0] == magicA9 {
		cnt, er := strconv.ParseInt(ptn[1], 10, 32)
		if er != nil {
			return
		}

		qc := countQuotePair(ptn[2]);
		if qc != int(cnt) {
			return
		}

		stm := ptn[2][qc : len(ptn[2])-qc]
		logDebug("deal runtime Env, exec sql=%s", stm)
		err = src.Query(func(row *sql.Rows) error {
			cols, er := row.ColumnTypes()
			if er != nil {
				return errorAndLog("failed to exe env, v=%s, er=%v", v, er)
			}

			ln := len(cols)
			vals := make([]interface{}, ln)
			ptrs := make([]interface{}, ln)
			for i := 0; i < ln; i++ {
				ptrs[i] = &vals[i]
			}

			if row.Next() {
				row.Scan(ptrs...)
				str, _ := src.Literal(vals[0], cols[0].DatabaseTypeName())
				rst = str
				tmp[v] = str
				has = true
			}

			return nil
		}, stm)

		if err != nil {
			return
		}

		if !has {
			err = errorAndLog("failed to exe env sql, v=%s", v)
		}
	}
	return
}

type exeStat struct {
	startd time.Time
	agreed bool
	valctx map[string]interface{}
	printf func(exe *Exe, str string, src bool)
	cnttop int64
	cntrow int64
	cntson int64
	cntdst int64
	cntsrc int64
}

func RunSqlx(pref *Preference, sqlx *SqlExe, src *MyConn, dst []*MyConn, risk bool) error {

	para := &exeStat{}
	para.startd = time.Now()
	para.agreed = risk
	para.valctx = make(map[string]interface{}) // 存放select的REF

	CtrlRoom.putEnv(roomTreeEnvSqlx, sqlx)
	CtrlRoom.putEnv(roomTreeEnvStat, para)

	tmp := make(map[string]string)
	for k, v := range sqlx.Envs {
		r, h, e := stmtEnv(v, src, tmp);
		if e != nil {
			return e
		}
		if h {
			logDebug("put runtime Env, hld=%s, val=%s", k, r)
			para.valctx[k] = r
		} else {
			para.valctx[k] = v
		}
	}

	ncm, lcm, dlt := "\n"+pref.LineComment+" ", pref.LineComment, pref.DelimiterRaw
	para.printf = func(exe *Exe, sql string, src bool) {

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

		one, end, ech := actOneEndEch(exe)
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
		er := runExe(exe, src, dst, para, 1)
		scnd := time.Now().Sub(para.startd).Seconds()
		LogTrace("stats time=%.2fs, tree/s=%.2f, src/s=%.2f, dst/s=%.2f, trees=%d, select-row=%d, child-exe=%d, src-affect=%d, dst-affect=%d",
			scnd, float64(para.cnttop)/scnd, float64(para.cntsrc)/scnd, float64(para.cntdst)/scnd,
			para.cnttop, para.cntrow, para.cntson,
			para.cntsrc, para.cntdst)

		if er != nil {
			return er
		}
	}

	return nil
}

var defValCol = regexp.MustCompile(`(VAL|COL)\[([^\[\]]*)\]`)

func runExe(exe *Exe, src *MyConn, dst []*MyConn, para *exeStat, lvl int) error {

	// 判断数据源和执行条件
	if arg, igr := skipHasNotRun(src, exe.Acts, para.valctx); igr {
		logDebug("SKIP exe on Condition. arg=%d seg=%d", arg.Head, exe.Seg.Head)
		return nil
	}

	// 构造执行语句
	stmt, prnt, vals, err := buildStatement(exe, para.valctx, src)
	if err != nil {
		return err
	}

	// 运行时变量 "SRC-DBNAME"
	dbsName := src.DbName()
	stmt = strings.Replace(stmt, magicDs, dbsName, -1)
	prnt = strings.Replace(prnt, magicDs, dbsName, -1)

	var valOnv []int
	for i := 0; i < len(vals); i++ {
		if vals[i] == magicDs {
			vals[i] = dbsName
		} else if vals[i] == magicDo {
			valOnv = append(valOnv, i)
		}
	}

	para.printf(exe, prnt, true)
	para.printf(exe, prnt, false)

	head := exe.Seg.Head
	line := exe.Seg.Line

	jobx := true // 保证执行
	defer func() {
		if jobx {
			CtrlRoom.dealJobx(nil, head)
		}
	}()

	logDebug("take stmt, id=%d, lvl=%d line=%s, stmt=%q", head, lvl, line, stmt)
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
				para.cntrow++
				jobx = true
				LogTrace("loop %d-th row, id=%d, line=%s", cnt, head, line)
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
									para.valctx[hld] = cln
									logDebug("simple sys DEF, hold=%s, para=%s, col-name=%s", hld, ptn, cln)
								} else { // VAL
									para.valctx[hld] = vals[j]
									dbt := cols[j].DatabaseTypeName()
									para.valctx[hld+":DatabaseTypeName"] = dbt
									logDebug("simple sys DEF, hold=%s, para=%s, value=%#v, dbtype=%s", hld, ptn, vals[j], dbt)
								}
							} else {
								pld := fmt.Sprintf("%s:%d", hld, k) // 保证多值的不能直接找到
								if sub[1] == "COL" {
									cls := make([]string, ln)
									for i, c := range cols {
										cls[i] = c.Name()
									}
									para.valctx[pld] = cls
									logDebug("simple sys DEF, hold=%s, para=%s, values'count=%d", pld, ptn, len(cls))
								} else {
									dbt := make([]string, ln)
									for i, c := range cols {
										dbt[i] = c.DatabaseTypeName()
									}
									para.valctx[pld] = vals
									para.valctx[pld+":DatabaseTypeName"] = dbt
									logDebug("simple sys DEF, hold=%s, para=%s, value'count=%d", pld, ptn, len(dbt))
								}
							}
						}
					}

					for i := 0; lost && i < ln; i++ {
						if strings.EqualFold(cols[i].Name(), ptn) {
							para.valctx[hld] = vals[i]
							dbt := cols[i].DatabaseTypeName()
							para.valctx[hld+":DatabaseTypeName"] = dbt
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
				bsn := false
				for _, son := range exe.Sons {
					if !shouldForAct(son, cnt) {
						continue
					}
					logDebug("fork ONE/FOR child=%d, parent=%d, lvl=%d", son.Seg.Head, head, lvl+1)
					er := runExe(son, src, dst, para, lvl+1)
					if er != nil {
						return er
					}
					bsn = true
				}

				if bsn { // 有zi
					para.cntson++
					if lvl == 1 {
						para.cnttop++
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
					logDebug("fork END child=%d, parent=%d, lvl=%d", son.Seg.Head, head, lvl+1)
					er := runExe(son, src, dst, para, lvl+1)
					if er != nil {
						return er
					}
				}
			}

			LogTrace("loop %d rows, id=%d, lvl=%d, line=%s", cnt, head, lvl, line)

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
		if para.agreed {
			if rsrc {
				logDebug("running on SRC db=%s", dbsName)
				if a, e := src.Exec(stmt, vals...); e != nil {
					LogError("failed on SRC=%s, id=%d, lvl=%d err=%v", dbsName, head, lvl, e)
					return e
				} else {
					para.cntsrc = para.cntsrc + a
					LogTrace("affect %d on SRC=%s, id=%d, lvl=%d", a, dbsName, head, lvl)
				}
			}
			// 单线程，出错停止
			if rout {
				for i, db := range dst {
					dboName := db.DbName()
					otmt := strings.Replace(stmt, magicDo, dboName, -1);
					for _, d := range valOnv {
						logDebug("replace out-db at %d with %s", d, dboName)
						vals[d] = dboName
					}
					logDebug("running on OUT[%d/%d] db=%s", i+1, dcnt, dboName)
					if a, e := db.Exec(otmt, vals...); e != nil {
						LogError("failed on [%d/%d]OUT=%s, id=%d, lvl=%d, err=%v", i+1, dcnt, dboName, head, lvl, e)
						return e
					} else {
						para.cntdst = para.cntdst + a
						LogTrace("affect %d on [%d/%d]OUT=%s, id=%d, lvl=%d", a, i+1, dcnt, dboName, head, lvl)
					}
				}
			}
		} else {
			if rsrc {
				LogTrace("fake run on SRC db=%s", dbsName)
			}

			if rout {
				hevo := strings.Contains(stmt, magicDo)
				for i, db := range dst {
					odn := db.DbName()
					for _, d := range valOnv {
						LogTrace("replace OUT-DB at index=%d with %s", d+1, odn)
					}
					LogTrace("fake run on OUT[%d/%d] db=%s", i+1, dcnt, odn)
					if hevo {
						LogTrace("replace runtime ENV. stmt=%s", strings.Replace(stmt, magicDo, odn, -1))
					}
				}
			}
		}
	}

	logDebug("done stmt, id=%d, lvl=%d, line=%s\n", head, lvl, line)
	return nil
}

func skipHasNotRun(con *MyConn, args []*Arg, ctx map[string]interface{}) (*Arg, bool) {

	for _, arg := range args {
		if arg.Hold == HoldTop {
			return arg, false
		}

		par := arg.Para
		if par == ParaHas || par == ParaNot {
			va := ctx[arg.Hold]
			no := con.Nothing(va)
			//
			if par == ParaHas && no {
				return arg, true
			}
			if par == ParaNot && !no {
				return arg, true
			}
		}

		if par == ParaOne || par == ParaFor || par == ParaEnd {
			va := ctx[arg.Hold]
			if va == nil {
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

func actOneEndEch(exe *Exe) (one, end, ech bool) {
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

	one, end, ech := actOneEndEch(exe)

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

	one, end, ech := actOneEndEch(exe)

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
