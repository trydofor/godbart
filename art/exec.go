package art

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

func Exec(pref *Preference, dest []*DataSource, file []FileEntity, risk bool) error {

	stmc, flln, cnln := 0, len(file), len(dest)
	sqls := make([]*Sqls, flln)

	// 解析和计算执行语句
	for i, f := range file {
		sql := ParseSqls(pref, &f)
		for _, v := range sql {
			if v.Exeb {
				stmc++
			}
		}
		sqls[i] = &sql
	}

	log.Printf("[TRACE] exec statements, sqls=%d, files=%d\n", stmc, flln)

	// 打开链接
	wg := &sync.WaitGroup{}
	conn := make([]*MyConn, cnln)
	for i, v := range dest {
		con, er := openDbAndLog(v)
		if er != nil {
			return errorAndLog("failed to open db=%s, err=%#v", v.Code, er)
		}
		conn[i] = con
		wg.Add(1)
	}

	// 多库并发，单库有序
	for i := 0; i < cnln; i++ {
		cur, con := 0, conn[i]

		var runner = func() {
			defer wg.Done()
			for j := 0; j < flln; j++ {
				pcnt, sqlj := 0, sqls[j]
				log.Printf("[TRACE] exec db=%s, file=%s\n", con.DbName(), file[j].Path)

				for _, sql := range *sqlj {
					if sql.Exeb {
						pcnt++
					}
				}
				execEach(pref, sqlj, con, cur, stmc, risk)
				cur = cur + pcnt
			}
		}

		if risk {
			go runner()
		} else {
			runner()
		}
	}

	wg.Wait()
	return nil
}

func execEach(pref *Preference, sqls *Sqls, conn Conn, cur, cnt int, risk bool) {
	cmn, dlt := pref.LineComment, pref.DelimiterRaw
	for _, sql := range *sqls {
		if sql.Exeb {
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
	if cur != cnt {
		log.Printf("[TRACE] db=%s, %d/%d, partly done\n\n", conn.DbName(), cur, cnt)
	} else {
		log.Printf("[TRACE] db=%s, sqls=%d, whole done\n\n", conn.DbName(), cnt)
	}
}
