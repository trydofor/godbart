package internal

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
)

type ReviSeg struct {
	revi string
	segs []Seg
}

func Revi(pref *Preference, dest []*DataSource, file []FileEntity, revi string, mask string, test bool) (err error) {

	mreg, err := regexp.Compile(mask)
	if err != nil {
		log.Fatalf("[ERROR] failed to compile mask=%s, err=%v\n", mask, err)
		return
	}

	reviSegs, reviFind, reviCurr := []ReviSeg{}, false, ""
	var reviSlt, reviUdp string
	// 倒序分版本块
	for k := len(file) - 1; k >= 0; k-- {
		f := file[k]
		log.Printf("[TRACE] revi file=%s\n", f.Path)
		var sqls *SqlSeg // err is shadowed during return
		sqls, err = ParseSqls(pref, &f)
		if err != nil {
			log.Fatalf("[ERROR] failed to parse sql, err=%v\n", err)
			return
		}

		// 按版本分段
		segs := sqls.Segs
		numRevi, idxRevi := "", len(segs)-1

		var reviSplit = func(i int) {
			v := segs[i]
			// find and check SELECT REVI
			for j := i; j < idxRevi; j++ {
				w := segs[j]
				if w.Type == SegRow {
					if len(reviSlt) == 0 {
						reviSlt = w.Text
						log.Printf("[TRACE] find SLT-REVI-SQL, file=%s, line=%s, sql=%s\n", w.File, w.Line, w.Text)
					} else {
						if reviSlt != w.Text {
							s := fmt.Sprintf("[ERROR] SLT-REVI-SQL changed, file=%s, line=%s, sql=%s\n", w.File, w.Line, w.Text)
							log.Fatal(s)
							err = errors.New(s)
							return
						}
					}
					break
				}
			}

			if strings.Compare(numRevi, revi) > 0 {
				log.Printf("[TRACE] IGNORE bigger revi=%s, file=%s, line=%s\n", numRevi, v.File, v.Line)
			} else {
				reviSegs = append(reviSegs, ReviSeg{numRevi, segs[i+1 : idxRevi+1]})
				log.Printf("[TRACE] ADD candidate revi=%s, file=%s, line=%s\n", numRevi, v.File, v.Line)
			}
		}

		for i := idxRevi; i >= 0; i-- {
			v := segs[i]
			if v.Type == SegExe {
				if r := findUpdRevi(v.Text, reviUdp, mreg); len(r) > 0 {
					if len(reviUdp) == 0 { // first
						log.Printf("[TRACE] find UPD-REVI-SQL, revi=%s, file=%s, line=%s, sql=%s\n", r, v.File, v.Line, v.Text)
						p := strings.Index(v.Text, r)
						reviUdp = trimUpdRevi(v.Text[0:p])
					} else {
						log.Printf("[TRACE] find more revi=%s, file=%s, line=%s\n", r, v.File, v.Line)
					}

					if len(reviCurr) == 0 {
						reviCurr = r
					} else {
						if strings.Compare(reviCurr, r) <= 0 {
							s := fmt.Sprintf("[ERROR] need uniq&asc revi, but %s <= %s. file=%s, line=%s, sql=%s\n", reviCurr, r, v.File, v.Line, v.Text)
							log.Fatal(s)
							err = errors.New(s)
							return
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
		s := fmt.Sprintf("[ERROR] can not find assigned revi=%s\n", revi)
		log.Fatal(s)
		err = errors.New(s)
		return
	}

	lastIdx := len(reviSegs) - 1
	if lastIdx < 0 {
		s := fmt.Sprintf("[ERROR] no sqls to run for revi=%s\n", revi)
		log.Fatal(s)
		err = errors.New(s)
		return
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
		conn := &MyConn{}
		log.Printf("[TRACE] trying Db=%s\n", v.Code)
		err = conn.Open(pref, v)

		if err != nil {
			log.Fatalf("[ERROR] failed to open db=%s, err=%v\n", v.Code, err)
			continue
		}
		log.Printf("[TRACE] opened Db=%s\n", v.Code)

		wg.Add(1)
		if test {
			goRevi(wg, reviSegs, conn, reviSlt, mreg, test)
		} else {
			go goRevi(wg, reviSegs, conn, reviSlt, mreg, test)
		}

	}

	wg.Wait()
	return
}

var blankRegexp = regexp.MustCompile("[ \t\r\n]+")

func trimUpdRevi(str string) string {
	lower := strings.ToLower(str)
	return blankRegexp.ReplaceAllString(lower, "")
}

func findUpdRevi(updSeg string, updRevi string, mask *regexp.Regexp) (revi string) {
	if len(updRevi) > 0 && !strings.HasPrefix(trimUpdRevi(updSeg), updRevi) { // 判断相似度
		return
	}

	// 判断规则
	return mask.FindString(updSeg)
}

func goRevi(wg *sync.WaitGroup, segs []ReviSeg, conn Conn, slt string, mask *regexp.Regexp, test bool) {
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
				return errors.New(fmt.Sprintf("[ERROR] revi not matched. revi=%s on db=%s use sql=%s\n", revi, conn.DbName(), slt))
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

			if test {
				log.Printf("[DEBUG] db=%s, %d/%d, TEST, NOT run. revi=%s, file=%s ,line=%s\n", conn.DbName(), p, c, s.revi, v.File, v.Line)
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
