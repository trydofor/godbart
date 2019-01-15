package art

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type DiffItem struct {
	ColArr []string
	ColMap map[string]Col
	IdxArr []string
	IdxMap map[string]Idx
	TrgArr []string
	TrgMap map[string]Trg
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

	stbl, sset, err := makeDiffTbl(scon, rgx)
	sdnm := scon.DbName()
	if err != nil {
		LogFatal("failed to list tables, db=%s, err=%v", sdnm, err)
		return err
	}

	detail := kind != DiffTbl
	hastrg := kind == DiffAll
	sdtl := make(map[string]DiffItem)

	for _, con := range dcon {
		dtbl, dset, er := makeDiffTbl(con, rgx)
		ddnm := con.DbName()
		if er != nil {
			LogFatal("failed to list tables, db=%s, err=%v", ddnm, er)
			return er
		}
		LogTrace("=== diff tbname ===, left=%s, right=%s", sdnm, ddnm)

		rep, ch := strings.Builder{}, true
		var ih []string // 两库都有，交集
		head := fmt.Sprintf("\n#TBNAME LEFT(>)=%s, RIGHT(<)=%s", sdnm, ddnm)
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

		sort.Strings(ih) // 排序

		if detail {
			LogTrace("=== diff detail ===, left=%s, right=%s", sdnm, ddnm)

			e1 := makeDiffAll(scon, ih, sdtl, hastrg) // 比较多库，逐步添加表
			if e1 != nil {
				return e1
			}

			ddtl := make(map[string]DiffItem) // 当前比较项
			e2 := makeDiffAll(con, ih, ddtl, hastrg)
			if e2 != nil {
				return e2
			}

			diffAll(ih, sdtl, ddtl, &rep, sdnm, ddnm)
		}

		if rep.Len() > 0 {
			LogTrace("== HAS SOME DIFF ==. LEFT=%s, RIGHT=%s", sdnm, ddnm)
			OutTrace(rep.String())
		} else {
			LogTrace("== ALL THE SAME ==. LEFT=%s, RIGHT=%s", sdnm, ddnm)
		}
	}

	return nil
}

func diffCol(ld, rd DiffItem, rep *strings.Builder) {
	if len(ld.ColArr) == 0 && len(rd.ColArr) == 0 {
		return
	}

	la, ra := ld.ColArr, rd.ColArr
	lm, rm := ld.ColMap, rd.ColMap

	tit := "=Col Only Name"
	fc := len(tit)
	for k := range lm {
		i := len(k)
		if i > fc {
			fc = i
		}
	}

	off := len(lm) - len(rm)

	pad := fmt.Sprintf("%d", fc)
	head := "\n" + tit + strings.Repeat(" ", fc-len(tit)) + " | No. | Type | Nullable | Default | Comment | Extra"
	null := "<NULL>"
	fmto := "\n%-" + pad + "s | %3d | %s | %t | %s | %s | %s"
	fmtb := "\n%-" + pad + "s | %s | %s | %s | %s | %s | %s"

	fmth := func(c *Col, tok string) {
		dvl := null
		if c.Deft.Valid {
			dvl = c.Deft.String
		}
		rep.WriteString(fmt.Sprintf(fmto, tok+c.Name, c.Seq, c.Type, c.Null, dvl, c.Cmnt, c.Extr))
	}

	var ic []Col
	ch := true
	for _, c := range la {
		li :=  lm[c]
		ri, ok := rm[c]
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
	for _, c := range ra {
		ri := rm[c]
		_, ok := lm[c]
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
			rep.WriteString(fmt.Sprintf(fmtb, "!"+li.Name, seq, typ, nul, dft, cmt, ext))
		}
	}
}

func diffIdx(ld, rd DiffItem, rep *strings.Builder) {
	if len(ld.IdxArr) == 0 && len(rd.IdxArr) == 0 {
		return
	}

	la, ra := ld.IdxArr, rd.IdxArr
	lm, rm := ld.IdxMap, rd.IdxMap

	ch, ih := true, true

	tit := "=Idx Only Name"
	fc := len(tit)
	for k := range lm {
		i := len(k)
		if i > fc {
			fc = i
		}
	}

	pad := fmt.Sprintf("%d", fc)
	head := "\n" + tit + strings.Repeat(" ", fc-len(tit)) + " | Uniq | Type | Cols"
	fmto := "\n%-" + pad + "s | %t | %s | %s"
	fmtb := "\n%-" + pad + "s | %s | %s | %s"

	var ic []Idx
	for _, c := range la {
		li := lm[c]
		ri, ok := lm[c]
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
	for _, c := range ra {
		ri := rm[c]
		_, ok := lm[c]
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
			rep.WriteString(fmt.Sprintf(fmtb, "!"+li.Name, typ, unq, cols))
		}
	}
}

func diffTrg(ld, rd DiffItem, rep *strings.Builder) {
	if len(ld.TrgArr) == 0 && len(rd.TrgArr) == 0 {
		return
	}

	la, ra := ld.TrgArr, rd.TrgArr
	lm, rm := ld.TrgMap, rd.TrgMap


	ch, ih := true, true

	tit := "=Trg Only Name"
	fc := len(tit)
	for k := range lm {
		i := len(k)
		if i > fc {
			fc = i
		}
	}

	pad := fmt.Sprintf("%d", fc)
	head := "\n" + tit + strings.Repeat(" ", fc-len(tit)) + " | Timing | Event | Statement"
	fmto := "\n%-" + pad + "s | %s | %s | %q"
	fmtb := "\n%-" + pad + "s | %s | %s | %s"

	var ic []Trg
	for _, c := range la {
		li := lm[c]
		ri, ok := rm[c]
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
	for _,c := range ra {
		ri := rm[c]
		_, ok := lm[c]
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
			rep.WriteString(fmt.Sprintf(fmtb, "!"+li.Name, tim, evt, stm))
		}
	}
}

func trimStatement(str string) string {
	str = squashBlank(str)
	str = squashTrimx(str)
	str = strings.ToLower(str)
	return str
}

func diffAll(tbl []string, lit, rit map[string]DiffItem, rep *strings.Builder, ldb, rdb string) {
	// 右侧是左侧的子集
	for _, tb := range tbl {
		ld, rd := lit[tb], rit[tb]
		sb := &strings.Builder{}
		// column
		diffCol(ld, rd, sb)
		// index
		diffIdx(ld, rd, sb)
		// trigger
		diffTrg(ld, rd, sb)

		if sb.Len() > 0 {
			rep.WriteString(fmt.Sprintf("\n#DETAIL TABLE=%s, LEFT(>)=%s, RIGHT(<)=%s", tb, ldb, rdb))
			rep.WriteString(sb.String())
		}
	}
}

func makeDiffTbl(conn *MyConn, rgx []*regexp.Regexp) (rst []string, set map[string]bool, err error) {
	rst, err = listTable(conn, rgx)
	if err != nil {
		return
	}
	sort.Strings(rst)

	set = make(map[string]bool)
	for _, v := range rst {
		set[v] = true
	}

	return
}

func makeDiffAll(con *MyConn, tbl []string, dtl map[string]DiffItem, trg bool) error {
	for _, t := range tbl {
		_, ok := dtl[t]
		if !ok {
			var cla, ixa, tga []string

			clm, err := con.Columns(t)
			if err != nil {
				LogError("failed to list columns, table=%s, db=%s, err=%v", t, con.DbName(), err)
				return err
			}
			if ln := len(clm); ln > 0 {
				tmp := make([]Col, 0, ln)
				for _, v := range clm {
					tmp = append(tmp, v)
				}
				sort.Slice(tmp, func(i, j int) bool {
					return tmp[i].Seq < tmp[j].Seq
				})
				cla = make([]string, ln)
				for i, v := range tmp {
					cla[i] = v.Name
				}
			}

			ixm, err := con.Indexes(t)
			if err != nil {
				LogError("failed to list indexes, table=%s, db=%s, err=%v", t, con.DbName(), err)
				return err
			}
			if ln := len(ixm); ln > 0 {
				ixa = make([]string, 0, ln)
				for k := range ixm {
					ixa = append(ixa, k)
				}
				sort.Strings(ixa)
			}

			var tgm map[string]Trg
			if trg {
				tgm, err = con.Triggers(t)
				if err != nil {
					LogError("failed to list triggers, table=%s, db=%s, err=%v", t, con.DbName(), err)
					return err
				}
				if ln := len(tgm); ln > 0 {
					tga = make([]string, 0, ln)
					for k := range tgm {
						tga = append(tga, k)
					}
					sort.Strings(tga)
				}
			}

			dtl[t] = DiffItem{cla, clm, ixa, ixm, tga, tgm}
		}
	}
	return nil
}

func showCreate(pref *Preference, conn *MyConn, rgx []*regexp.Regexp) {
	tbs, err := listTable(conn, rgx)
	if err != nil {
		return
	}

	dbn := conn.DbName()
	drw := pref.DelimiterRaw
	dcm := pref.DelimiterCmd
	dlc := pref.LineComment

	if len(tbs) == 0 {
		LogTrace("no tables on db=%s, err=%v", dbn, err)
		return
	}

	sort.Strings(tbs)

	c := len(tbs)
	for i, tn := range tbs {
		LogTrace("db=%s, %d/%d, table=%s", dbn, i+1, c, tn)
		tb, e := conn.DdlTable(tn)
		if e != nil {
			LogError("db=%s, failed to dll table=%s", dbn, tn)
		} else {
			ddl := fmt.Sprintf("DROP TABLE IF EXISTS `%s`%s\n%s%s\n", tn, drw, tb, drw)
			OutTrace("%s db=%s, %d/%d, table=%s\n%s", dlc, dbn, i+1, c, tn, ddl)
		}

		tgs, e := conn.Triggers(tn)
		if e != nil {
			LogError("db=%s, failed to get triggers=%s", dbn, tn)
		} else {
			if cnt := len(tgs); cnt > 0 {
				tns := make([]string, 0, cnt)
				for k := range tgs {
					tns = append(tns, k)
				}
				sort.Strings(tns)

				for _, k := range tns {
					tg, r := conn.DdlTrigger(k)
					if r != nil {
						LogError("db=%s, failed to ddl trigger=%s, table=%s", dbn, k, tn)
					} else {
						ddl := fmt.Sprintf("DROP TRIGGER IF EXISTS `%s` %s\n%s $$\n%s $$\n%s %s\n", k, drw, dcm, tg, dcm, drw)
						OutTrace("%s db=%s, trigger=%s, table=%s\n%s", dlc, dbn, k, tn, ddl)
					}
				}
			}
		}
	}

	return
}
