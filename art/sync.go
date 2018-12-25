package art

import (
	"log"
	"regexp"
)

const (
	SyncTbl = "tbl" // 同步表和索引
	SyncAll = "all" // 同步说有
	SyncTrg = "trg" // 同步trigger
)

var SyncKind = []string{SyncTbl, SyncAll, SyncTrg}

func Sync(srce *DataSource, dest []*DataSource, kind string, rgx []*regexp.Regexp) error {

	if srce == nil {
		return errorAndLog("[ERROR] need source db to diff, kind=%s\n", kind)
	}

	scon, err := openDbAndLog(srce)
	if err != nil {
		return err
	}

	// 要执行的 ddl
	var name, ddls []string

	// 获得所有表
	tbls, err := listTable(scon, rgx);
	if err != nil {
		return err
	}

	if kind == SyncAll || kind == SyncTbl {
		for _, v := range tbls {
			ddl, er := scon.DdlTable(v)
			if er != nil {
				return er
			}
			name = append(name, "table="+v)
			ddls = append(ddls, ddl)
			log.Printf("[TRACE] %4d ddl table=%s\n", len(tbls), v)
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
				log.Printf("[TRACE] %4d ddl trigger=%s\n", len(tbls), k)
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
			_, er := conn.Exec(v)
			if er != nil {
				log.Printf("[ERROR] %d/%d failed on db=%s, name=%s, err=%v\n", i+1, cnt, db.Code, name[i], er)
			} else {
				log.Printf("[TRACE] %d/%d done db=%s, name=%s\n", i+1, cnt, db.Code, name[i])
			}
		}
	}

	return nil
}
