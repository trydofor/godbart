package internal

import (
	"log"
	"sync"
)

func Exec(pref *Preference, dest []DataSource, file []FileEntity, test bool) (err error) {

	for _, f := range file {
		log.Printf("[TRACE] exec file=%s\n", f.Path)
		var sqls *SqlSeg // err is shadowed during return
		sqls, err = ParseSqls(pref, &f)
		if err != nil {
			log.Fatalf("[ERROR] failed to parse sql, err=%v\n", err)
			return
		}

		wg := &sync.WaitGroup{}
		for _, v := range dest {
			conn := &MyConn{}
			log.Printf("[TRACE] trying Db=%s\n", v.Code)
			err = conn.Open(pref, &v)

			if err != nil {
				log.Fatalf("[ERROR] failed to open db=%s, err=%v\n", v.Code, err)
				continue
			}
			log.Printf("[TRACE] opened Db=%s\n", v.Code)

			wg.Add(1)
			if test {
				goExec(wg, sqls, conn, test)
			} else {
				go goExec(wg, sqls, conn, test)
			}

		}

		wg.Wait()
	}
	return
}

func goExec(wg *sync.WaitGroup, sqd *SqlSeg, conn Conn, test bool) {
	defer wg.Done()
	c := len(sqd.Segs)
	for i, v := range sqd.Segs {
		p := i + 1

		if v.Type != SegCmt {
			if test {
				log.Printf("[DEBUG] db=%s, %d/%d, TEST, NOT run.\n-- file=%s ,line=%s\n%s\n", conn.DbName(), p, c, v.File, v.Line, v.Text)
				continue
			}

			cnt, err := conn.Exec(v.Text)
			if err != nil {
				log.Fatalf("[ERROR] db=%s, %d/%d, failed to exec sql, file=%s, line=%s, err=%v\n", conn.DbName(), p, c, v.File, v.Line, err)
				break
			} else {
				log.Printf("[TRACE] db=%s, %d/%d, %d affects. file=%s, line=%s\n", conn.DbName(), p, c, cnt, v.File, v.Line)
			}
		}
	}
}
