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

type DiffItem struct {
	Columns  map[string]Col
	Indexes  map[string]Idx
	Triggers map[string]Trg
}

var DiffKinds = []string{TbName, Detail, Create}

func Diff(pref *Preference, srce *DataSource, dest []*DataSource, kind string, rgx []*regexp.Regexp) (err error) {

	log.Printf("[TRACE] ===== use `grep -vE '^[0-9]{4}'` to filter =====\n")

	if kind == Create {
		dbs := make([]*DataSource, 0, len(dest)+1)
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

	stbl, sset, err := makeTbname(scon, rgx)
	if err != nil {
		log.Fatalf("[ERROR] failed to list tables, db=%s, err=%v\n", scon.DbName(), err)
		return err
	}

	detail := kind == Detail
	sdtl := make(map[string]DiffItem)

	for _, d := range dcon {
		dtbl, dset, err := makeTbname(d, rgx)
		if err != nil {
			log.Fatalf("[ERROR] failed to list tables, db=%s, err=%v\n", d.DbName(), err)
			return err
		}
		log.Printf("[TRACE] === diff tbname ===, left=%s, right=%s\n", scon.DbName(), d.DbName())

		rep, ch := strings.Builder{}, true
		var ih []string
		head := fmt.Sprintf("\n#TBNAME LEFT(>)=%s, RIGHT(<)=%s", scon.DbName(), d.DbName())
		for _, k := range stbl {
			_, ok := dset[k]
			if ok {
				ih = append(ih, k)
			} else {
				if ch {
					ch = false
					rep.WriteString(head)
				}
				rep.WriteString("\n>")
				rep.WriteString(k)
			}
		}

		for _, k := range dtbl {
			_, ok := sset[k]
			if !ok {
				if ch {
					ch = false
					rep.WriteString(head)
				}

				rep.WriteString("\n<")
				rep.WriteString(k)
			}
		}

		if detail {
			log.Printf("[TRACE] === diff detail ===, left=%s, right=%s\n", scon.DbName(), d.DbName())

			err = makeDetail(scon, ih, sdtl) // 比较多库，逐步添加表
			if err != nil {
				return err
			}
			ddtl := make(map[string]DiffItem) // 当前比较项
			err = makeDetail(d, ih, ddtl)
			if err != nil {
				return err
			}

			diffDetail(sdtl, ddtl, &rep, scon.DbName(), d.DbName())
		}

		if rep.Len() > 0 {
			log.Println(rep.String())
		} else {
			log.Printf("== ALL THE SAME ==. LEFT=%s, RIGHT=%s\n", scon.DbName(), d.DbName())
		}
	}

	return
}

func makeDetail(con *MyConn, tbl []string, dtl map[string]DiffItem) error {
	for _, t := range tbl {
		_, ok := dtl[t]
		if !ok {
			cls, err := con.Columns(t)
			if err != nil {
				log.Fatalf("[ERROR] failed to list columns, table=%s, db=%s, err=%v\n", t, con.DbName(), err)
				return err
			}
			ixs, err := con.Indexes(t)
			if err != nil {
				log.Fatalf("[ERROR] failed to list indexes, table=%s, db=%s, err=%v\n", t, con.DbName(), err)
				return err
			}

			tgs, err := con.Triggers(t)
			if err != nil {
				log.Fatalf("[ERROR] failed to list triggers, table=%s, db=%s, err=%v\n", t, con.DbName(), err)
				return err
			}

			dtl[t] = DiffItem{cls, ixs, tgs}
		}
	}
	return nil
}

func diffCol(lc, rc map[string]Col, rep *strings.Builder) {
	var ic []Col
	// 左侧有，右侧没有

	tit := "=Col Only Name"
	fc := len(tit)
	for k := range lc {
		i := len(k)
		if i > fc {
			fc = i
		}
	}

	off := len(lc) - len(rc)

	pad := fmt.Sprintf("%d", fc)
	head := "\n" + tit + strings.Repeat(" ", fc-len(tit)) + " | No. | Type | Nullable | Default | Comment | Extra"
	null := "<NULL>"
	fmto := "\n%-" + pad + "s | %3d | %s | %t | %s | %s | %s"
	fmtb := "\n%-" + pad + "s | %s | %s | %s | %s | %s | %s"

	fmth := func(c *Col, tok string) {
		d := null
		if c.Deft.Valid {
			d = c.Deft.String
		}
		rep.WriteString(fmt.Sprintf(fmto, tok+c.Name, c.Seq, c.Type, c.Null, d, c.Key, c.Cmnt, c.Extr))
	}

	ch := true
	for c, li := range lc {
		ri, ok := rc[c]
		if ok {
			ic = append(ic, li, ri)
		} else {
			if ch {
				rep.WriteString(head)
				ch = false
			}
			fmth(&li, ">")
		}
	}

	// 右侧有，左侧没有
	for c, ri := range rc {
		_, ok := lc[c]
		if !ok {
			if ch {
				rep.WriteString(head)
				ch = false
			}
			fmth(&ri, "<")
		}
	}

	// 比较两者都有的
	ih := true
	for i := 0; i < len(ic); i = i + 2 {
		li, ri := ic[i], ic[i+1]
		var seq, typ, nul, dft, cmt, ext string
		cnt := 0

		if li.Seq == ri.Seq || li.Seq-ri.Seq == off {
			seq = fmt.Sprintf("%3d", li.Seq)
		} else {
			seq = fmt.Sprintf("%d:%d", li.Seq, ri.Seq)
			cnt++
		}

		if li.Type != ri.Type {
			typ = fmt.Sprintf("%s:%s", li.Type, ri.Type)
			cnt++
		}

		if li.Null != ri.Null {
			nul = fmt.Sprintf("%t:%t", li.Null, ri.Null)
			cnt++
		}

		if (!li.Deft.Valid && !ri.Deft.Valid) || (li.Deft.Valid && ri.Deft.Valid && li.Deft.String == ri.Deft.String) {
			// equals
		} else {
			ln, rn := null, null
			if li.Deft.Valid {
				ln = li.Deft.String
			}
			if ri.Deft.Valid {
				rn = ri.Deft.String
			}
			dft = fmt.Sprintf("`%s`:`%s`", ln, rn)
			cnt++
		}

		if li.Cmnt != ri.Cmnt {
			cmt = fmt.Sprintf("%s:%s", li.Cmnt, ri.Cmnt)
			cnt++
		}

		if li.Extr != ri.Extr {
			ext = fmt.Sprintf("%s:%s", li.Extr, ri.Extr)
			cnt++
		}

		if cnt > 0 {
			if ih {
				ih = false
				rep.WriteString(strings.Replace(head, "Only", "Diff", 1))
			}
			rep.WriteString(fmt.Sprintf(fmtb, li.Name, seq, typ, nul, dft, cmt, ext))
		}
	}
}

func diffIdx(lc, rc map[string]Idx, rep *strings.Builder) {
	var ic []Idx
	// 左侧有，右侧没有
	ch, ih := true, true

	tit := "=Idx Only Name"
	fc := len(tit)
	for k := range lc {
		i := len(k)
		if i > fc {
			fc = i
		}
	}

	pad := fmt.Sprintf("%d", fc)
	head := "\n" + tit + strings.Repeat(" ", fc-len(tit)) + " | Uniq | Type | Cols"
	fmto := "\n%-" + pad + "s | %t | %s | %s"
	fmtb := "\n%-" + pad + "s | %s | %s | %s"

	for c, li := range lc {
		ri, ok := rc[c]
		if ok {
			ic = append(ic, li, ri)
		} else {
			if ch {
				ch = false
				rep.WriteString(head)
			}
			rep.WriteString(fmt.Sprintf(fmto, ">"+li.Name, li.Uniq, li.Type, li.Cols))
		}
	}

	// 右侧有，左侧没有
	for c, ri := range rc {
		_, ok := lc[c]
		if !ok {
			if ch {
				ch = false
				rep.WriteString(head)
			}
			rep.WriteString(fmt.Sprintf(fmto, "<"+ri.Name, ri.Uniq, ri.Type, ri.Cols))
		}
	}

	// 比较两者都有的
	for i := 0; i < len(ic); i = i + 2 {
		li, ri := ic[i], ic[i+1]
		var typ, unq, cols string
		cnt := 0

		if li.Type != ri.Type {
			typ = fmt.Sprintf("%s:%s", li.Type, ri.Type)
			cnt++
		}

		if li.Uniq != ri.Uniq {
			unq = fmt.Sprintf("%t:%t", li.Uniq, ri.Uniq)
			cnt++
		}

		if li.Cols != ri.Cols {
			cols = fmt.Sprintf("%s:%s", li.Cols, ri.Cols)
			cnt++
		}

		if cnt > 0 {
			if ih {
				ih = false
				rep.WriteString(strings.Replace(head, "Only", "Diff", 1))
			}
			rep.WriteString(fmt.Sprintf(fmtb, li.Name, typ, unq, cols))
		}
	}
}

func diffTrg(lc, rc map[string]Trg, rep *strings.Builder) {
	var ic []Trg
	// 左侧有，右侧没有
	ch, ih := true, true

	tit := "=Trg Only Name"
	fc := len(tit)
	for k := range lc {
		i := len(k)
		if i > fc {
			fc = i
		}
	}

	pad := fmt.Sprintf("%d", fc)
	head := "\n" + tit + strings.Repeat(" ", fc-len(tit)) + " | Timing | Event | Statement"
	fmto := "\n%-" + pad + "s | %s | %s | %q"
	fmtb := "\n%-" + pad + "s | %s | %s | %s"

	for c, li := range lc {
		ri, ok := rc[c]
		if ok {
			ic = append(ic, li, ri)
		} else {
			if ch {
				ch = false
				rep.WriteString(head)
			}
			rep.WriteString(fmt.Sprintf(fmto, ">"+li.Name, li.Timing, li.Event, li.Statement))
		}
	}

	// 右侧有，左侧没有
	for c, ri := range rc {
		_, ok := lc[c]
		if !ok {
			if ch {
				ch = false
				rep.WriteString(head)
			}
			rep.WriteString(fmt.Sprintf(fmto, "<"+ri.Name, ri.Timing, ri.Event, ri.Statement))
		}
	}
	// 比较两者都有的
	for i := 0; i < len(ic); i = i + 2 {
		li, ri := ic[i], ic[i+1]
		var tim, evt, stm string
		cnt := 0
		if li.Timing != ri.Timing {
			tim = fmt.Sprintf("%s:%s", li.Timing, ri.Timing)
			cnt++
		}
		if li.Event != ri.Event {
			evt = fmt.Sprintf("%s:%s", li.Event, ri.Event)
			cnt++
		}
		if trimStatement(li.Statement) != trimStatement(ri.Statement) {
			stm = fmt.Sprintf("%q:%q", li.Statement, ri.Statement)
			cnt++
		}

		if cnt > 0 {
			if ih {
				ih = false
				rep.WriteString(strings.Replace(head, "Only", "Diff", 1))
			}
			rep.WriteString(fmt.Sprintf(fmtb, li.Name, tim, evt, stm))
		}
	}
}

func trimStatement(str string) string {
	str = squashBlank(str)
	str = squashTrimx(str)
	str = strings.ToLower(str)
	return str
}

func diffDetail(lit, rit map[string]DiffItem, rep *strings.Builder, ldb, rdb string) {
	// 右侧是左侧的子集
	for tb, rd := range rit {
		ld := lit[tb]
		sb := &strings.Builder{}
		// column
		diffCol(ld.Columns, rd.Columns, sb)
		// index
		diffIdx(ld.Indexes, rd.Indexes, sb)
		// trigger
		diffTrg(ld.Triggers, rd.Triggers, sb)

		if sb.Len() > 0 {
			rep.WriteString(fmt.Sprintf("\n#DETAIL TABLE=%s, LEFT(>)=%s, RIGHT(<)=%s", tb, ldb, rdb))
			rep.WriteString(sb.String())
		}
	}
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

	for _, t := range tbs {
		for _, r := range rgx {
			if r.MatchString(t) {
				rst = append(rst, t)
			}
		}
	}
	return
}
