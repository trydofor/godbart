package art

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type DiffItem struct {
	Columns  map[string]Col
	Indexes  map[string]Idx
	Triggers map[string]Trg
}

func Diff(pref *Preference, srce *DataSource, dest []*DataSource, kind string, rgx []*regexp.Regexp) error {

	if srce == nil {
		return errorAndLog("need source db to diff, type=%s", kind)
	}

	if kind == DiffDdl {
		dbs := make([]*DataSource, 0, len(dest)+1)
		dbs = append(dbs, srce)
		dbs = append(dbs, dest...)

		if len(dbs) == 0 {
			return errorAndLog("no db to show create")
		}

		for _, db := range dbs {
			conn, er := openDbAndLog(db)
			if er != nil {
				return er
			}
			showCreate(pref, conn, rgx)
		}
		return nil
	}

	scon, err := openDbAndLog(srce)
	if err != nil {
		return err
	}

	dcon := make([]*MyConn, len(dest))
	for i, db := range dest {
		conn, er := openDbAndLog(db)
		if er != nil {
			return er
		}
		dcon[i] = conn
	}

	stbl, sset, err := makeTbname(scon, rgx)
	if err != nil {
		LogFatal("failed to list tables, db=%s, err=%v", scon.DbName(), err)
		return err
	}

	detail := kind == DiffAll
	sdtl := make(map[string]DiffItem)

	for _, con := range dcon {
		dtbl, dset, er := makeTbname(con, rgx)
		if er != nil {
			LogFatal("failed to list tables, db=%s, err=%v", con.DbName(), er)
			return er
		}
		LogTrace("=== diff tbname ===, left=%s, right=%s", scon.DbName(), con.DbName())

		rep, ch := strings.Builder{}, true
		var ih []string
		head := fmt.Sprintf("\n#TBNAME LEFT(>)=%s, RIGHT(<)=%s", scon.DbName(), con.DbName())
		for _, tbl := range stbl {
			if dset[tbl] {
				ih = append(ih, tbl)
			} else {
				if ch {
					ch = false
					rep.WriteString(head)
				}
				rep.WriteString("\n>")
				rep.WriteString(tbl)
			}
		}

		for _, tbl := range dtbl {
			if !sset[tbl] {
				if ch {
					ch = false
					rep.WriteString(head)
				}

				rep.WriteString("\n<")
				rep.WriteString(tbl)
			}
		}

		if detail {
			LogTrace("=== diff detail ===, left=%s, right=%s", scon.DbName(), con.DbName())

			e1 := makeDetail(scon, ih, sdtl) // 比较多库，逐步添加表
			if e1 != nil {
				return e1
			}

			ddtl := make(map[string]DiffItem) // 当前比较项
			e2 := makeDetail(con, ih, ddtl)
			if e2 != nil {
				return e2
			}

			diffDetail(sdtl, ddtl, &rep, scon.DbName(), con.DbName())
		}

		if rep.Len() > 0 {
			LogTrace("== HAS SOME DIFF ==. LEFT=%s, RIGHT=%s", scon.DbName(), con.DbName())
			OutTrace(rep.String())
		} else {
			LogTrace("== ALL THE SAME ==. LEFT=%s, RIGHT=%s", scon.DbName(), con.DbName())
		}
	}

	return nil
}

func makeDetail(con *MyConn, tbl []string, dtl map[string]DiffItem) error {
	for _, t := range tbl {
		_, ok := dtl[t]
		if !ok {
			cls, err := con.Columns(t)
			if err != nil {
				LogError("failed to list columns, table=%s, db=%s, err=%v", t, con.DbName(), err)
				return err
			}
			ixs, err := con.Indexes(t)
			if err != nil {
				LogError("failed to list indexes, table=%s, db=%s, err=%v", t, con.DbName(), err)
				return err
			}

			tgs, err := con.Triggers(t)
			if err != nil {
				LogError("failed to list triggers, table=%s, db=%s, err=%v", t, con.DbName(), err)
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

func makeTbname(conn *MyConn, rgx []*regexp.Regexp) (rst []string, set map[string]bool, err error) {
	rst, err = listTable(conn, rgx)
	if err != nil {
		return
	}

	set = make(map[string]bool)
	for _, v := range rst {
		set[v] = true
	}

	return
}

func showCreate(pref *Preference, conn *MyConn, rgx []*regexp.Regexp) {
	tbs, err := listTable(conn, rgx)
	if err != nil {
		return
	}

	if len(tbs) == 0 {
		LogTrace("no tables on db=%s, err=%v", conn.DbName(), err)
		return
	}

	sort.Strings(tbs)

	c := len(tbs)
	for i, v := range tbs {
		LogTrace("db=%s, %d/%d, table=%s", conn.DbName(), i+1, c, v)
		tb, e := conn.DdlTable(v)
		if e != nil {
			LogError("db=%s, failed to dll table=%s", conn.DbName(), v)
		} else {
			ddl := fmt.Sprintf("DROP TABLE IF EXISTS `%s`%s\n%s%s\n", v, pref.DelimiterRaw, tb, pref.DelimiterRaw)
			OutTrace("%s db=%s, %d/%d, table=%s\n%s", pref.LineComment, conn.DbName(), i+1, c, v, ddl)
		}

		tgs, e := conn.Triggers(v)
		if e != nil {
			LogError("db=%s, failed to get triggers=%s", conn.DbName(), v)
		} else {
			for _, g := range tgs {
				tg, r := conn.DdlTrigger(g.Name)
				if r != nil {
					LogError("db=%s, failed to ddl trigger=%s, table=%s", conn.DbName(), g.Name, v)
				} else {
					ddl := fmt.Sprintf("DROP TRIGGER IF EXISTS `%s` %s\n%s $$\n%s $$\n%s %s\n", g.Name, pref.DelimiterRaw, pref.DelimiterCmd, tg, pref.DelimiterCmd, pref.DelimiterRaw)
					OutTrace("%s db=%s, trigger=%s, table=%s\n%s", pref.LineComment, conn.DbName(), g.Name, v, ddl)
				}
			}
		}
	}

	return
}

func listTable(conn *MyConn, rgx []*regexp.Regexp) (rst []string, err error) {

	var tbs []string
	tbs, err = conn.Tables()
	if err != nil {
		LogError("failed to show tables db=%s, err=%v", conn.DbName(), err)
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
