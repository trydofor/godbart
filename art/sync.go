package art

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

func Sync(srce *DataSource, dest []*DataSource, kind string, rgx []*regexp.Regexp) error {

	if srce == nil {
		return errorAndLog("need source db to diff, type=%s", kind)
	}

	scon, err := openDbAndLog(srce)
	if err != nil {
		return err
	}

	// 要执行的 ddl
	var name, ddls []string

	// 获得所有表
	tbls, err := listTable(scon, rgx)
	if err != nil {
		return err
	}

	if kind == SyncRow {
		type udp struct {
			tbln string
			stms string
			vals []interface{}
		}

		chns := make([]chan *udp, len(dest))

		wg := &sync.WaitGroup{}
		for i, db := range dest {
			conn, er := openDbAndLog(db)
			if er != nil {
				return er
			}

			chns[i] = make(chan *udp, 5)
			idb, icn := db.Code, chns[i]
			wg.Add(1)
			go func() {
				for i := 1; ; i++ {
					u := <-icn
					if len(u.stms) == 0 {
						LogTrace("end %d rows database=%s", i, idb)
						wg.Done()
						return
					}
					a, e := conn.Exec(u.stms, u.vals...)
					if e != nil {
						LogError("failed to sync %d-th row on db=%s, table=%s, err=%v", i, idb, u.tbln, e)
					} else {
						LogDebug("inserted %d-th row affects %d, db=%s, table=%s", i, a, idb, u.tbln)
					}
				}
			}()
		}

		tbln := len(tbls)
		for i, v := range tbls {
			LogTrace("%d/%d tables", i+1, tbln)
			var ff = func(row *sql.Rows) error {
				cols, er := row.Columns()
				if er != nil {
					return er
				}

				for ln, cnt := len(cols), 1; row.Next(); cnt++ {
					vals := make([]interface{}, ln)
					ptrs := make([]interface{}, ln)
					for i := 0; i < ln; i++ {
						ptrs[i] = &vals[i]
					}

					row.Scan(ptrs...)
					u := &udp{
						v,
						fmt.Sprintf("insert into %s values(%s)", v, strings.Repeat(",?", ln)[1:]),
						vals,
					}
					LogDebug("sync %d row of table=%s", cnt, v)
					for _, c := range chns {
						c <- u
					}
				}
				return nil
			}
			er := scon.Query(ff, "select * from "+v)
			if er != nil {
				LogError("sync data failed, table=%s, err=%v", v, er)
				return er
			}
		}
		// END
		u := &udp{}
		for _, c := range chns {
			c <- u
		}
		LogTrace("waiting for sync done")
		wg.Wait()
		return nil
	}

	if kind == SyncAll || kind == SyncTbl {
		for _, v := range tbls {
			ddl, er := scon.DdlTable(v)
			if er != nil {
				return er
			}
			name = append(name, "table="+v)
			ddls = append(ddls, ddl)
			LogTrace("%4d ddl table=%s", len(ddls), v)
		}
	}

	if kind == SyncAll || kind == SyncTrg {
		for _, v := range tbls {
			tgs, er := scon.Triggers(v)
			if er != nil {
				return er
			}
			for k := range tgs {
				ddl, er := scon.DdlTrigger(k)
				if er != nil {
					return er
				}
				name = append(name, "trigger="+k)
				ddls = append(ddls, ddl)
				LogTrace("%4d ddl trigger=%s", len(ddls), k)
			}
		}
	}

	cnt := len(ddls)
	for _, db := range dest {
		conn, er := openDbAndLog(db)
		if er != nil {
			return er
		}

		for i, v := range ddls {
			_, e2 := conn.Exec(v)
			if e2 != nil {
				LogError("%4d/%d failed on db=%s, name=%s, err=%v", i+1, cnt, db.Code, name[i], e2)
			} else {
				LogTrace("%4d/%d done db=%s, name=%s", i+1, cnt, db.Code, name[i])
			}
		}
	}

	return nil
}
