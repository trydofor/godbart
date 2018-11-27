package internal

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
)

const (
	TbName = "tbname" // 分别对比`-s`和多个`-d` 间的表名差异
	Detail = "detail" // 分别对比`-s`和多个`-d` 间的表明细(column, index,trigger)
	Create = "create" // 生成多库的创建DDL(table&index，trigger)
)

var DiffKinds = []string{TbName, Detail, Create}

func Diff(pref *Preference, srce *DataSource, dest []*DataSource, kind string, rgx []*regexp.Regexp) (err error) {

	log.Printf("[TRACE] ===== use `grep -vE '^[0-9]{4}'` to filter =====\n")

	if kind == Create {
		dbs := []*DataSource{}
		dbs = append(dbs, srce)
		dbs = append(dbs, dest...)

		if len(dbs) == 0 {
			log.Fatalf("[ERROR] no db to show create\n")
			return
		}

		for _, db := range dbs {
			conn := &MyConn{}
			log.Printf("[TRACE] trying Db=%s\n", db.Code)
			err = conn.Open(pref, db)
			if err != nil {
				log.Fatalf("[ERROR] failed to open db=%s, err=%v\n", conn.DbName(), err)
				return
			}
			showCreate(conn, rgx)
		}
		return
	}

	if srce == nil {
		s := fmt.Sprintf("[ERROR] need source db to diff, kind=%s\n", kind)
		log.Fatal(s)
		err = errors.New(s)
		return
	}

	scon := &MyConn{}
	log.Printf("[TRACE] trying source Db=%s\n", srce.Code)
	err = scon.Open(pref, srce)
	if err != nil {
		log.Fatalf("[ERROR] failed to open srouce db=%s, err=%v\n", scon.DbName(), err)
		return
	}

	dcon := make([]*MyConn, len(dest))
	for i, d := range dest {
		conn := &MyConn{}
		log.Printf("[TRACE] trying destination Db=%s\n", d.Code)
		err = conn.Open(pref, d)
		if err != nil {
			log.Fatalf("[ERROR] failed to open destination db=%s, err=%v\n", conn.DbName(), err)
			return
		}
		dcon[i] = conn
	}

	if kind == TbName {
		stbl, sset, err := makeTbname(scon, rgx)
		if err != nil {
			log.Fatalf("[ERROR] failed to list tables, db=%s, err=%v\n", scon.DbName(), err)
			return err
		}

		for _, d := range dcon {
			dtbl, dset, err := makeTbname(d, rgx)
			if err != nil {
				log.Fatalf("[ERROR] failed to list tables, db=%s, err=%v\n", d.DbName(), err)
				return err
			}
			log.Printf("[TRACE] === diff === tbname, left=%s, right=%s\n", scon.DbName(), d.DbName())

			rep, lh, rh := strings.Builder{}, true, true
			for _, k := range stbl {
				_, ok := dset[k]
				if !ok {
					if lh {
						rep.WriteString("\n== left only ==, left=" + scon.DbName() + ", right=" + d.DbName())
						lh = false
					}
					rep.WriteString("\n")
					rep.WriteString(k)
				}
			}

			for _, k := range dtbl {
				_, ok := sset[k]
				if !ok {
					if rh {
						rep.WriteString("\n== right only ==, left=" + scon.DbName() + ", right=" + d.DbName())
						rh = false
					}

					rep.WriteString("\n")
					rep.WriteString(k)
				}
			}
			if rep.Len() > 0 {
				log.Println(rep.String())
			} else {
				log.Printf("== Same ==. left=%s, right=%s\n", scon.DbName(), d.DbName())
			}
		}
	}

	return
}

func makeTbname(conn *MyConn, rgx []*regexp.Regexp) (rst []string, set map[string]*string, err error) {
	rst, err = listTable(conn, rgx)
	if err != nil {
		return
	}

	set = make(map[string]*string)
	for _, v := range rst {
		set[v] = nil
	}

	return
}

func showCreate(conn *MyConn, rgx []*regexp.Regexp) {
	tbs, err := listTable(conn, rgx)
	if err != nil {
		return
	}

	if len(tbs) == 0 {
		log.Printf("[TRACE] no tables on db=%s, err=%v\n", conn.DbName(), err)
		return
	}

	sort.Strings(tbs)

	c := len(tbs)
	for i, v := range tbs {
		log.Printf("[TRACE] db=%s, %d/%d, table=%s\n", conn.DbName(), i+1, c, v)
		tb, e := conn.DdlTable(v)
		if e != nil {
			log.Fatalf("[ERROR] db=%s, failed to dll table=%s\n", conn.DbName(), v)
		} else {
			log.Printf("[TRACE] \n-- db=%s, %d/%d, table=%s\n%s", conn.DbName(), i+1, c, v, tb)
		}

		tgs, e := conn.Triggers(v)
		if e != nil {
			log.Fatalf("[ERROR] db=%s, failed to get triggers=%s\n", conn.DbName(), v)
		} else {
			for _, g := range tgs {
				tg, r := conn.DdlTrigger(g.Name)
				if r != nil {
					log.Fatalf("[ERROR] db=%s, failed to ddl trigger=%s, table=%s\n", conn.DbName(), g.Name, v)
				} else {
					log.Printf("[TRACE] \n-- db=%s, trigger=%s, table=%s\n%s", conn.DbName(), g.Name, v, tg)
				}
			}
		}
	}

	return
}

func listTable(conn *MyConn, rgx []*regexp.Regexp) (rst []string, err error) {

	tbs, err := conn.Tables()
	if err != nil {
		log.Fatalf("[ERROR] failed to show tables db=%s, err=%v\n", conn.DbName(), err)
		return
	}

	if len(tbs) == 0 || len(rgx) == 0 {
		return tbs, nil
	}

	rst = []string{}
	for _, t := range tbs {
		for _, r := range rgx {
			if r.MatchString(t) {
				rst = append(rst, t)
			}
		}
	}
	return
}
