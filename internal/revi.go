package internal

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
)

type ReviSeg struct {
	revi string
	segs []Sql
}

func Revi(pref *Preference, dest []*DataSource, file []FileEntity, revi string, mask string, risk bool) error {

	mreg, err := regexp.Compile(mask)
	if err != nil {
		log.Fatalf("[ERROR] failed to compile mask=%s, err=%v\n", mask, err)
		return err
	}

	var reviSegs []ReviSeg
	reviFind, reviCurr := false, ""
	var reviSlt, reviUdp string

	// 倒序分版本块
	for k := len(file) - 1; k >= 0; k-- {
		f := file[k]
		log.Printf("[TRACE] revi file=%s\n", f.Path)
		sqls := ParseSqls(pref, &f)

		// 按版本分段
		numRevi, idxRevi := "", len(sqls)-1
		var reviSplit = func(i int) {
			v := sqls[i]
			// find and check SELECT REVI
			for j := i; j < idxRevi; j++ {
				w := sqls[j]
				if w.Type == SegRow {
					if len(reviSlt) == 0 {
						reviSlt = w.Text
						log.Printf("[TRACE] find SLT-REVI-SQL, file=%s, line=%s, sql=%s\n", w.File, w.Line, w.Text)
					} else {
						if reviSlt != w.Text {
							err = errorAndLog("[ERROR] SLT-REVI-SQL changed, file=%s, line=%s, sql=%s\n", w.File, w.Line, w.Text)
							return
						}
					}
					break
				}
			}

			if strings.Compare(numRevi, revi) > 0 {
				log.Printf("[TRACE] IGNORE bigger revi=%s, file=%s, line=%s\n", numRevi, v.File, v.Line)
			} else {
				reviSegs = append(reviSegs, ReviSeg{numRevi, sqls[i+1 : idxRevi+1]})
				log.Printf("[TRACE] ADD candidate revi=%s, file=%s, line=%s\n", numRevi, v.File, v.Line)
			}
		}

		for i := idxRevi; i >= 0; i-- {
			v := sqls[i]
			if v.Type == SegExe {
				if r := findUpdRevi(v.Text, reviUdp, mreg); len(r) > 0 {
					if len(reviUdp) == 0 { // first
						log.Printf("[TRACE] find UPD-REVI-SQL, revi=%s, file=%s, line=%s, sql=%s\n", r, v.File, v.Line, v.Text)
						p := strings.Index(v.Text, r)
						reviUdp = strings.ToLower(removeWhite(v.Text[0:p]))
					} else {
						log.Printf("[TRACE] find more revi=%s, file=%s, line=%s\n", r, v.File, v.Line)
					}

					if len(reviCurr) == 0 {
						reviCurr = r
					} else {
						if strings.Compare(reviCurr, r) <= 0 {
							return errorAndLog("[ERROR] need uniq&asc revi, but %s <= %s. file=%s, line=%s, sql=%s\n", reviCurr, r, v.File, v.Line, v.Text)
						}
					}

					if revi == r {
						log.Printf("[TRACE] find DONE revi=%s, file=%s, line=%s\n", r, v.File, v.Line)
						reviFind = true
					}

					if i < idxRevi {
						reviSplit(i)
					}

					idxRevi = i
					numRevi = r
				}
			}

			if i == 0 {
				reviSplit(i)
			}
		}
	}

	if !reviFind {
		return errorAndLog("[ERROR] can not find assigned revi=%s\n", revi)
	}

	lastIdx := len(reviSegs) - 1
	if lastIdx < 0 {
		return errorAndLog("[ERROR] no sqls to run for revi=%s\n", revi)
	}

	if len(reviSlt) == 0 {
		log.Printf("[TRACE] without SLT-REVI-SQL, means run all revi all\n")
	}

	// reverse
	for i, j := 0, lastIdx; i < j; i, j = i+1, j-1 {
		reviSegs[i], reviSegs[j] = reviSegs[j], reviSegs[i]
	}

	// run
	wg := &sync.WaitGroup{}
	for _, v := range dest {
		conn, er := openDbAndLog(v)
		if er != nil {
			continue
		}

		wg.Add(1)
		if risk {
			go goRevi(wg, reviSegs, conn, reviSlt, mreg, risk)
		} else {
			goRevi(wg, reviSegs, conn, reviSlt, mreg, risk)
		}
	}

	wg.Wait()
	return nil
}

func findUpdRevi(updSeg string, updRevi string, mask *regexp.Regexp) (revi string) {
	if len(updRevi) > 0 && !strings.HasPrefix(strings.ToLower(removeWhite(updSeg)), updRevi) { // 判断相似度
		return
	}

	// 判断规则
	return mask.FindString(updSeg)
}

func goRevi(wg *sync.WaitGroup, segs []ReviSeg, conn Conn, slt string, mask *regexp.Regexp, risk bool) {
	defer wg.Done()

	sc := len(segs)
	log.Printf("[TRACE] find %d revis to run on db=%s\n", sc, conn.DbName())

	var revi string
	var sn = func(rs *sql.Rows) (err error) {
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
				return errorAndLog(fmt.Sprintf("[ERROR] revi not matched. revi=%s on db=%s use sql=%s\n", revi, conn.DbName(), slt))
			}
		} else {
			log.Printf("[TRACE] get NULL revi on db=%s use sql=%s\n", conn.DbName(), slt)
		}

		return
	}

	err := conn.Query(sn, slt)
	if err != nil {
		if strings.Contains(fmt.Sprintf("%v", err), "Error 1146") {
			log.Printf("[TRACE] Table not exist on db=%s use sql=%s, err=%v\n", conn.DbName(), slt, err)
		} else {
			log.Fatalf("[ERROR] failed to select revision on db=%s use sql=%s, err=%v\n", conn.DbName(), slt, err)
			return
		}
	}

	log.Printf("[TRACE] get revi=%s on db=%s use sql=%s\n", revi, conn.DbName(), slt)

	// run
	for j, s := range segs {

		if len(revi) > 0 && strings.Compare(s.revi, revi) <= 0 {
			log.Printf("[TRACE] ===Run=== ignore smaller. db=%s, %d/%d, revi=%s, db-revi=%s\n", conn.DbName(), j+1, sc, s.revi, revi)
			continue
		}

		c := len(s.segs)
		log.Printf("[TRACE] ===Run=== db=%s, %d/%d, revi=%s\n", conn.DbName(), j+1, sc, s.revi)

		for i, v := range s.segs {
			p := i + 1

			if v.Type == SegCmt {
				continue
			}

			if !risk {
				fmt.Printf("\n-- db=%s, %d/%d, revi=%s, file=%s ,line=%s\n%s", conn.DbName(), p, c, s.revi, v.File, v.Line, v.Text)
				continue
			}

			cnt, err := conn.Exec(v.Text)
			if err != nil {
				log.Fatalf("[ERROR] db=%s, %d/%d, failed to revi sql, revi=%s, file=%s, line=%s, err=%v\n", conn.DbName(), p, c, s.revi, v.File, v.Line, err)
				break
			} else {
				log.Printf("[TRACE] db=%s, %d/%d, %d affects. revi=%s, file=%s, line=%s\n", conn.DbName(), p, c, cnt, s.revi, v.File, v.Line)
			}
		}
	}
}
