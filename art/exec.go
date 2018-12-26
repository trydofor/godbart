package art

import (
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

	LogTrace("exec statements, sql-count=%d, file-count=%d", stmc, flln)

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
				LogTrace("exec db=%s, file=%s", con.DbName(), file[j].Path)

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
					OutTrace("%s find '%s', May Need '%s' to avoid", cmn, dlt, pref.DelimiterCmd)
				}
				OutTrace("%s db=%s, %3d/%d, id=%3d, line=%s, file=%s", cmn, conn.DbName(), cur, cnt, sql.Head, sql.Line, sql.File)
				OutDebug("%s%s", sql.Text, dlt)
				continue
			}

			a, err := conn.Exec(sql.Text)
			if err != nil {
				LogError("db=%s, %3d/%d, failed to exec sql, id=%3d, line=%s, file=%s, err=%v", conn.DbName(), cur, cnt, sql.Head, sql.Line, sql.File, err)
				break
			} else {
				LogTrace("db=%s, %d/%d, %d affects. id=%3d, line=%s, file=%s", conn.DbName(), cur, cnt, a, sql.Head, sql.Line, sql.File)
			}
		}
	}
	if cur != cnt {
		LogTrace("db=%s, %d/%d, partly done", conn.DbName(), cur, cnt)
	} else {
		LogTrace("db=%s, sqls=%d, whole done", conn.DbName(), cnt)
	}
}
