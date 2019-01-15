package art

import (
	"strings"
	"sync"
)

func Exec(pref *Preference, dest []*DataSource, file []FileEntity, risk bool) error {

	cnte, cntf, cntd := 0, len(file), len(dest)
	var exes []*Exe

	// 解析和计算执行语句
	envs := make(map[string]string)
	for _, f := range file {
		sqls := ParseSqls(pref, &f)
		sqlx, er := ParseSqlx(sqls, envs)
		if er != nil {
			return er
		}
		exes = append(exes, sqlx.Exes...)
	}

	walkExes(exes, func(exe *Exe) error {
		cnte++
		return nil
	})

	LogTrace("exec statements, sql-count=%d, file-count=%d", cnte, cntf)

	// 打开链接
	wg := &sync.WaitGroup{}
	conn := make([]*MyConn, cntd)
	for i, v := range dest {
		con, er := openDbAndLog(v)
		if er != nil {
			return errorAndLog("failed to open db=%s, err=%#v", v.Code, er)
		}
		conn[i] = con
		wg.Add(1)
	}

	// 多库并发，单库有序
	cnt := 0
	walkExes(exes, func(exe *Exe) error {
		cnt ++
		return nil
	})

	cmn, dlt := pref.LineComment, pref.DelimiterRaw
	for _, con := range conn {
		cur, ddn := 0, con.DbName()
		ctx := make(map[string]interface{})
		gogo := func() {
			defer wg.Done()
			pureRunExes(exes, ctx, con, func(exe *Exe, stm string) error {
				cur++
				sql := exe.Seg
				if risk {
					a, err := con.Exec(stm)
					if err != nil {
						LogError("db=%s, %3d/%d, failed to exec sql, id=%3d, line=%s, file=%s, err=%v", ddn, cur, cnt, sql.Head, sql.Line, sql.File, err)
						return err
					} else {
						LogTrace("db=%s, %d/%d, %d affects. id=%3d, line=%s, file=%s", ddn, cur, cnt, a, sql.Head, sql.Line, sql.File)
					}
				} else {
					// 不处理 trigger 新结束符问题。
					if strings.Contains(stm, dlt) {
						OutTrace("%s find '%s', May Need '%s' to avoid", cmn, dlt, pref.DelimiterCmd)
					}
					OutTrace("%s db=%s, %3d/%d, id=%3d, line=%s, file=%s", cmn, ddn, cur, cnt, sql.Head, sql.Line, sql.File)
					OutDebug("%s%s", sql.Text, dlt)
				}

				return nil
			})
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
