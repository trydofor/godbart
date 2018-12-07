package art

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

func Exec(pref *Preference, dest []*DataSource, file []FileEntity, risk bool) error {

	fln := len(file)
	sqls := make([]*Sqls, 0, fln)
	cur, cnt := 0, 0
	for _, f := range file {
		sql := ParseSqls(pref, &f)
		for _, v := range sql {
			if v.Type != SegCmt {
				cnt++
			}
		}
		sqls = append(sqls, &sql)
	}

	log.Printf("[TRACE] exec statements, sqls=%d, files=%d\n", cnt, fln)

	for i := 0; i < fln; i++ {
		sqli := sqls[i]
		log.Printf("[TRACE] exec file=%s\n", file[i].Path)
		wg := &sync.WaitGroup{}
		for _, v := range dest {
			conn, er := openDbAndLog(v)
			if er != nil {
				continue
			}

			wg.Add(1)
			if risk {
				goExec(wg, pref, sqli, conn, cur, cnt, risk)
			} else {
				go goExec(wg, pref, sqli, conn, cur, cnt, risk)
			}
		}
		cur = cur + len(*sqli)
		wg.Wait()
	}

	return nil
}

func goExec(wg *sync.WaitGroup, pref *Preference, sqls *Sqls, conn Conn, cur, cnt int, risk bool) {
	defer wg.Done()
	cmn, dlt := pref.LineComment, pref.DelimiterRaw
	for _, sql := range *sqls {
		if sql.Type != SegCmt {
			cur++
			if !risk {
				// 不处理 trigger 新结束符问题。
				if strings.Contains(sql.Text, dlt) {
					fmt.Printf("\n%s find '%s', May Need '%s' to avoid", cmn, dlt, pref.DelimiterCmd)
				}
				fmt.Printf("\n%s db=%s, %d/%d, file=%s ,line=%s\n%s%s\n", cmn, conn.DbName(), cur, cnt, sql.File, sql.Line, sql.Text, dlt)
				continue
			}

			a, err := conn.Exec(sql.Text)
			if err != nil {
				log.Fatalf("[ERROR] db=%s, %d/%d, failed to exec sql, file=%s, line=%s, err=%v\n", conn.DbName(), cur, cnt, sql.File, sql.Line, err)
				break
			} else {
				log.Printf("[TRACE] db=%s, %d/%d, %d affects. file=%s, line=%s\n", conn.DbName(), cur, cnt, a, sql.File, sql.Line)
			}
		}
	}
	log.Printf("[TRACE] db=%s, %d/%d, this part is done\n\n", conn.DbName(), cur, cnt)
}
