package internal

import (
	"fmt"
	"log"
	"sync"
)

func Exec(pref *Preference, dest []*DataSource, file []FileEntity, risk bool) error {

	for _, f := range file {
		log.Printf("[TRACE] exec file=%s\n", f.Path)
		sqls := ParseSqls(pref, &f)

		wg := &sync.WaitGroup{}
		for _, v := range dest {
			conn, er := openDbAndLog(v)
			if er != nil {
				continue
			}

			wg.Add(1)
			if risk {
				goExec(wg, sqls, conn, risk)
			} else {
				go goExec(wg, sqls, conn, risk)
			}

		}

		wg.Wait()
	}
	return nil
}

func goExec(wg *sync.WaitGroup, sqls []Sql, conn Conn, risk bool) {
	defer wg.Done()
	c := len(sqls)
	for i, sql := range sqls {
		p := i + 1

		if sql.Type != SegCmt {
			if !risk {
				fmt.Printf("\n-- db=%s, %d/%d, file=%s ,line=%s\n%s", conn.DbName(), p, c, sql.File, sql.Line, sql.Text)
				continue
			}

			cnt, err := conn.Exec(sql.Text)
			if err != nil {
				log.Fatalf("[ERROR] db=%s, %d/%d, failed to exec sql, file=%s, line=%s, err=%v\n", conn.DbName(), p, c, sql.File, sql.Line, err)
				break
			} else {
				log.Printf("[TRACE] db=%s, %d/%d, %d affects. file=%s, line=%s\n", conn.DbName(), p, c, cnt, sql.File, sql.Line)
			}
		}
	}
}
