package art

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type ReviSeg struct {
	revi string
	exes []*Exe
}

func Revi(pref *Preference, dest []*DataSource, file []FileEntity, revi, mask, rqry string, risk bool) error {

	mreg, err := regexp.Compile(mask)
	if err != nil {
		LogFatal("failed to compile mask=%s, err=%v", mask, err)
		return err
	}

	if len(revi) == 0 || !mreg.MatchString(revi) {
		LogFatal("must assign revi number and match the mast")
		return err
	}

	var reviSegs []ReviSeg
	var reviSlt string
	var tknQry, tknSlt, tknUdp string

	reviFind, reviCurr := false, ""

	if len(rqry) == 0 {
		rqry = "SELECT"
	}
	tknQry = signifySql(rqry)

	// 倒序分版本块
	envs := make(map[string]string)
	for k := len(file) - 1; k >= 0; k-- {
		f := file[k]
		LogTrace("revi file=%s", f.Path)
		sqls := ParseSqls(pref, &f)

		// 按版本分段
		idxRevi := len(sqls) - 1
		reviSplit := func(bgn int, sqlRevi string) error {
			// find and check SELECT REVI
			ist := bgn // select-version-sql or bgn
			for j := bgn; j < idxRevi; j++ {
				if w := sqls[j]; w.Exeb {
					tkn := signifySql(w.Text)

					if !strings.HasPrefix(tkn, tknQry) {
						continue
					}

					if len(reviSlt) == 0 {
						reviSlt = w.Text
						tknSlt = tkn
						LogTrace("find SLT-REVI-SQL, line=%s, file=%s, sql=%s", w.Line, w.File, w.Text)
					} else {
						if tknSlt != tkn {
							return errorAndLog("SLT-REVI-SQL changed, first-sql=%s, file=%s, line=%s, now-		sql=%s", reviSlt, w.File, w.Line, w.Text)
						}
					}
					ist = j
					break
				}
			}

			v := sqls[ist]
			if strings.Compare(sqlRevi, revi) > 0 {
				LogTrace("IGNORE bigger revi=%s, line=%s, file=%s", sqlRevi, v.Line, v.File)
			} else {
				LogTrace("build revi=%s, line from=%d, to=%d, file=%s", sqlRevi, sqls[ist].Head, sqls[idxRevi].Head, v.File)
				exe, er := ParseSqlx(sqls[ist:idxRevi+1], envs)
				if er != nil {
					return er
				}
				reviSegs = append(reviSegs, ReviSeg{sqlRevi, exe.Exes})
				LogTrace("ADD candidate revi=%s, line from=%d, to=%d, file=%s", sqlRevi, sqls[ist].Head, sqls[idxRevi].Head, v.File)
			}
			return nil
		}

		numRevi := ""
		for i := idxRevi; i >= 0; i-- {
			v := sqls[i]
			if v.Exeb {
				stm := v.Text
				r := findUpdRevi(stm, tknUdp, mreg)

				if len(tknUdp) == 0 { // first
					if len(r) == 0 {
						return errorAndLog("REVI not matches in the last sql. line=%s, file=%s, sql=%s", v.Line, v.File, stm)
					}
					LogTrace("find UPD-REVI-SQL, revi=%s, line=%s, file=%s, sql=%s", r, v.Line, v.File, stm)
					p := strings.Index(stm, r)
					tknUdp = signifySql(stm[0:p])
				}

				if len(r) > 0 {
					LogTrace("find more revi=%s, line=%sfile=%s, ", r, v.Line, v.File)

					if len(reviCurr) == 0 {
						reviCurr = r
					} else {
						if strings.Compare(reviCurr, r) <= 0 {
							return errorAndLog("need uniq&asc revi, but %s <= %s. line=%s, file=%s, sql=%s", reviCurr, r, v.Line, v.File, stm)
						}
					}

					if revi == r {
						LogTrace("find DONE revi=%s, line=%s, file=%s", r, v.Line, v.File)
						reviFind = true
					}

					if i < idxRevi {
						if er := reviSplit(i, numRevi); er != nil {
							return er
						}
					}

					idxRevi = i
					numRevi = r
				}
			}

			if i == 0 {
				if er := reviSplit(0, numRevi); er != nil {
					return er
				}
			}
		}
	}

	if !reviFind {
		return errorAndLog("can not find assigned revi=%s", revi)
	}

	lastIdx := len(reviSegs) - 1
	if lastIdx < 0 {
		return errorAndLog("no sqls to run for revi=%s", revi)
	}

	if len(reviSlt) == 0 {
		LogTrace("without SLT-REVI-SQL, means run all revi all")
	}

	// reverse
	for i, j := 0, lastIdx; i < j; i, j = i+1, j-1 {
		reviSegs[i], reviSegs[j] = reviSegs[j], reviSegs[i]
	}

	// run
	// 打开链接
	wg := &sync.WaitGroup{}
	cnln := len(dest)
	conn := make([]*MyConn, cnln)
	for i, v := range dest {
		con, er := openDbAndLog(v)
		if er != nil {
			return errorAndLog("failed to open db=%s, err=%v", v.Code, er)
		}
		conn[i] = con
		wg.Add(1)
	}

	// 多库并发，单库有序
	for i := 0; i < cnln; i++ {
		con := conn[i]
		var gogo = func() {
			defer wg.Done()
			ReviEach(pref, reviSegs, con, reviSlt, mreg, risk)
		}
		if risk {
			go gogo()
		} else {
			gogo()
		}
	}

	wg.Wait()
	return nil
}

func findUpdRevi(updSeg string, tknUdp string, mask *regexp.Regexp) (revi string) {
	if len(tknUdp) > 0 && !strings.HasPrefix(signifySql(updSeg), tknUdp) { // 判断相似度
		return
	}

	// 判断规则
	return mask.FindString(updSeg)
}

func ReviEach(pref *Preference, revs []ReviSeg, conn *MyConn, slt string, mask *regexp.Regexp, risk bool) {

	var revi string
	dbn := conn.DbName()
	var slv = func(rs *sql.Rows) (err error) {
		var cols []string
		cols, err = rs.Columns()
		if err != nil || len(cols) != 1 {
			return
		}
		r1 := sql.NullString{}
		if rs.Next() {
			err = rs.Scan(&r1)
		}

		if r1.Valid {
			revi = r1.String
			if !mask.MatchString(revi) {
				return errorAndLog(fmt.Sprintf("revi not matched. revi=%s on db=%s use sql=%s", revi, dbn, slt))
			}
		} else {
			LogTrace("get NULL revi on db=%s use sql=%s", dbn, slt)
		}

		return
	}

	err := conn.Query(slv, slt)
	if err != nil {
		if conn.TableNotFound(err) {
			LogTrace("Table not exist, db=%s use sql=%s", dbn, slt)
		} else {
			LogError("failed to select revision on db=%s use sql=%s, err=v", dbn, slt, err)
			return
		}
	}

	if len(revi) == 0 {
		LogTrace("empty revi means always run. db=%s use sql=%s", dbn, slt)
	} else {
		LogTrace("get revi=%s on db=%s use sql=%s", revi, dbn, slt)
	}

	// run
	sts := make(map[string]bool)
	ctx := make(map[string]interface{})
	for _, s := range revs {
		walkExes(s.exes, func(exe *Exe) error {
			sts[fmt.Sprintf("%s:%d", exe.Seg.File, exe.Seg.Head)] = true
			return nil
		})
	}
	cnt := len(sts)
	lft := cnt

	cmn, dlt := pref.LineComment, pref.DelimiterRaw
	tkn := signifySql(slt)
	for _, s := range revs {

		pnt := 0
		if len(revi) > 0 && strings.Compare(s.revi, revi) <= 0 {
			walkExes(s.exes, func(exe *Exe) error {
				delete(sts, fmt.Sprintf("%s:%d", exe.Seg.File, exe.Seg.Head))
				pnt++
				return nil
			})

			LogTrace("ignore smaller. db=%s, revi=%s, db-revi=%s, sqls=[%d,%d]/%d", dbn, s.revi, revi, cnt-lft+1, cnt-lft+pnt, cnt)
			lft = lft - pnt
			continue
		} else {
			walkExes(s.exes, func(exe *Exe) error {
				pnt++
				return nil
			})
		}

		LogTrace("db=%s, revi=%s, sqls count=%d", dbn, s.revi, pnt)
		pureRunExes(s.exes, ctx, conn, func(exe *Exe, stm string) error {
			v := exe.Seg
			delete(sts, fmt.Sprintf("%s:%d", v.File, v.Head))
			lft = len(sts)
			if signifySql(stm) == tkn {
				LogTrace("db=%s, %d/%d. skip revi-slt. revi=%s, file=%s, line=%s", dbn, cnt-lft, cnt, s.revi, v.File, v.Line)
				return nil
			}
			if risk {
				a, err := conn.Exec(stm)
				if err != nil {
					LogError("db=%s, %d/%d, failed to revi sql, revi=%s, file=%s, line=%s, err=v", dbn, cnt-lft, cnt, s.revi, v.File, v.Line, err)
					return err
				} else {
					LogTrace("db=%s, %d/%d, %d affects. revi=%s, file=%s, line=%s", dbn, cnt-lft, cnt, a, s.revi, v.File, v.Line)
				}
			} else {
				// 不处理 trigger 新结束符问题。
				if strings.Contains(stm, dlt) {
					OutTrace("%s find '%s', May Need '%s' to avoid", cmn, dlt, pref.DelimiterCmd)
				}
				OutTrace("%s db=%s, %d/%d, revi=%s, file=%s ,line=%s\n%s%s", cmn, dbn, cnt-lft, cnt, s.revi, v.File, v.Line, stm, dlt)
			}
			return nil
		})

	}

	if lft == 0 {
		LogTrace("db=%s, exes=%d, all done", dbn, cnt)
	} else {
		LogTrace("db=%s, %d/%d, in progress", dbn, cnt-lft, cnt)
	}
}
