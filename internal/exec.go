package internal

import (
	"log"
	"sync"
)

func Exec(pref *Preference, dest []DataSource, file []FileEntity, test bool) (err error) {

	for _, f := range file {
		log.Printf("[TRACE] exec file=%s\n", f.Path)
		var sqd *SqlDyn
		sqd, err = ParseSql(pref, &f)
		if err != nil {
			log.Fatalf("[ERROR] failed to parse sql, err=%v\n", err)
			return
		}

		wg := &sync.WaitGroup{}
		for _, v := range dest {
			conn := &MyConn{}
			err = conn.Open(pref, &v)
			if err != nil {
				log.Fatalf("[ERROR] failed to open db=%s, err=%v\n", v.Code, err)
				continue
			}
			if test {
				for _, v := range sqd.Segs {
					if v.Type != COMMENT {
						log.Printf("[DEBUG] exec-test: print only, NOT run.\n-- db=%s ,file=%s ,line=%s\n%s\n", conn.DbName(), v.File, v.Line, v.Text)
					}
				}
			} else {
				wg.Add(1)
				go goExec(wg, sqd, conn)
			}
		}
		wg.Wait()
	}
	return
}

func goExec(wg *sync.WaitGroup, sqd *SqlDyn, conn Conn) {
	defer wg.Done()
	for _, v := range sqd.Segs {
		if v.Type != COMMENT {
			cnt, err := conn.Exec(v.Text)
			if err != nil {
				log.Fatalf("[ERROR] failed to exec sql on db=%s ,file=%s ,line=%s ,err=%v\n", conn.DbName(), v.File, v.Line, err)
				break
			} else {
				log.Printf("[TRACE] %d affects. db=%s ,file=%s ,line=%s\n", cnt, conn.DbName(), v.File, v.Line)
			}
		}
	}
}
