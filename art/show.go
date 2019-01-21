package art

import (
	"regexp"
	"sort"
	"strings"
)

const (
	ShowTblName = "${TABLE_NAME}"
	ShowTblDdl  = "${TABLE_DDL}"
	ShowTgrName = "${TRIGGER_NAME}"
	ShowTgrDdl  = "${TRIGGER_DDL}"
	ShowColBase = "${COLUMNS_BASE}"
	ShowColFull = "${COLUMNS_FULL}"
)

var ShowParaRgx = regexp.MustCompile(`\$\{(TABLE_NAME|TABLE_DDL|TRIGGER_NAME|TRIGGER_DDL|COLUMNS_BASE|COLUMNS_FULL)\}`)
var SqlSplitRgx = regexp.MustCompile(`[\r\n]+`)

type ShowTmpl struct {
	Key string          // 模板名
	Tpl string          // 原始模板
	Arg map[string]bool // 模板中的参数
	Idx []int           // 参数索引，[参数开始，参数结束，...]
}

func Show(srce *DataSource, ktpl []string, rgx []*regexp.Regexp) error {

	if srce == nil {
		return errorAndLog("need source db to show")
	}

	conn, err := openDbAndLog(srce)
	if err != nil {
		return err
	}

	tbls, err := listTable(conn, rgx)
	if err != nil {
		return err
	}
	if len(tbls) == 0 {
		LogTrace("no tables on db=%s", conn.DbName())
		return nil
	}

	sort.Strings(tbls)

	lns := len(ktpl)
	tpl := make([]ShowTmpl, 0, lns/2)
	for i := 0; i < lns; i += 2 {
		k, t := ktpl[i], ktpl[i+1]
		LogTrace("parse templet for key=%s", k)
		tpl = append(tpl, ParseTmpl(k, t))
	}

	dbn := conn.DbName()
	cnt := len(tbls)
	for j, tbl := range tbls {
		OutDebug("-- db=%s, %d/%d, table=%s", dbn, j+1, cnt, tbl)
		env := make(map[string]interface{})
		for i, p := range tpl {
			key := ktpl[i*2]
			LogTrace("merge templet for key=%s, table=%s", key, tbl)
			out, er := MergeTmpl(p, env, tbl, conn)
			if er != nil {
				LogError("failed to merge templet. key=%s, table=%s, err=%v", key, tbl, er)
				return er
			}
			OutTrace(out)
		}
	}

	return nil
}

func ParseTmpl(key, tpl string) ShowTmpl {
	mtc := ShowParaRgx.FindAllStringSubmatchIndex(tpl, -1)
	lns := len(mtc)
	arg := make(map[string]bool)
	idx := make([]int, 0, lns*2)
	for _, v := range mtc {
		idx = append(idx, v[0], v[1])
		arg[tpl[v[0]:v[1]]] = true
	}
	return ShowTmpl{key, tpl, arg, idx}
}

func MergeTmpl(tpl ShowTmpl, env map[string]interface{}, tbl string, con *MyConn) (string, error) {

	tm, zr := 1, 0
	for arg := range tpl.Arg {
		val, err := makeParaVal(arg, env, tbl, con)
		if err != nil {
			return "", err
		}
		switch arg {
		case ShowTgrName, ShowTgrDdl:
			if ln := len(val.([]string)); ln > 0 {
				tm = ln;
			} else {
				LogTrace("empty templet val, arg=%s", arg)
				zr++
			}
		default:
			if len(val.(string)) == 0 {
				LogTrace("empty templet val, arg=%s", arg)
				zr++
			}
		}
	}

	kln := len(tpl.Arg)
	if zr == kln {
		tm = 0
		LogDebug("skip all empty para templet, arg=%s", tpl.Key)
	} else if zr > 0 && zr < kln {
		return "", errorAndLog("partly empty templat val, arg=%s", tpl.Key)
	}

	var sb strings.Builder
	pln := len(tpl.Idx)
	tmp := tpl.Tpl
	for i := 0; i < tm; i++ {
		off := 0
		for j := 0; j < pln; j += 2 {
			b, e := tpl.Idx[j], tpl.Idx[j+1]
			if b > off {
				sb.WriteString(tmp[off:b])
			}
			key := tmp[b:e]
			off = e
			switch val := env[key]; val.(type) {
			case []string:
				sb.WriteString(val.([]string)[i])
			case string:
				sb.WriteString(val.(string))
			}
		}
		if off < len(tmp) {
			sb.WriteString(tmp[off:])
		}
	}

	return sb.String(), nil
}

func makeParaVal(key string, env map[string]interface{}, tbl string, con *MyConn) (interface{}, error) {
	if v, ok := env[key]; ok {
		return v, nil;
	}

	switch key {
	case ShowTblName:
		env[key] = tbl
		return tbl, nil
	case ShowTblDdl:
		if ddl, err := con.DdlTable(tbl); err == nil {
			env[key] = ddl
			return ddl, nil
		} else {
			return nil, err
		}
	case ShowTgrName:
		if trg, err := makeTrgList(tbl, con); err == nil {
			env[key] = trg
			return trg, nil
		} else {
			return nil, err
		}
	case ShowTgrDdl:
		var trg []string
		if val, ok := env[ShowTgrName]; ok {
			trg = val.([]string)
		} else {
			ntg, err := makeTrgList(tbl, con)
			if err == nil {
				trg = ntg
				env[ShowTgrName] = ntg
			} else {
				return nil, err
			}
		}

		ddl := make([]string, len(trg))
		for i, v := range trg {
			dl, err := con.DdlTrigger(v)
			if err != nil {
				return nil, err
			}
			ddl[i] = dl
		}
		env[key] = ddl
		return ddl, nil
	case ShowColBase:
		col, err := makeColList(tbl, con)
		if err != nil {
			return nil, err
		}

		fld := make([]string, len(col))
		for i, v := range col {
			fld[i] = v.Name + " " + v.Type
		}
		val := strings.Join(fld, ",\n")
		env[key] = val
		return val, nil
	case ShowColFull:
		var tmp string
		if val, ok := env[ShowTblDdl]; ok {
			tmp = val.(string)
		} else {
			if ddl, err := con.DdlTable(tbl); err == nil {
				tmp = ddl
				env[ShowTblDdl] = ddl
			} else {
				return nil, err
			}
		}

		col, err := makeColList(tbl, con)
		if err != nil {
			return nil, err
		}

		ddl := SqlSplitRgx.Split(tmp, -1)
		lnc, lnd := len(col), len(ddl)
		tkc, tkd := make([]string, lnc), make([]string, lnd)

		for i, c := range col {
			tkc[i] = strings.ToLower(onlyKeyChar(c.Name + c.Type))
		}
		for i, d := range ddl {
			tkd[i] = strings.ToLower(onlyKeyChar(d))
		}

		b := -1
		for c, d := 0, 0; c < lnc && d < lnd; d++ {
			if strings.HasPrefix(tkd[d], tkc[c]) {
				if b < 0 {
					b = d
				}
				c++
			} else {
				if c > 0 {
					return nil, errorAndLog("columns seq not matched")
				}
			}
		}

		val := strings.Join(ddl[b:b+lnc], "\n")
		// remote head or last `,`
		strings.TrimFunc(val, func(r rune) bool {
			return r == ','
		})
		env[key] = val
		return val, nil
	}

	return nil, errorAndLog("unsupported show para=%s", key)
}

func makeColList(tbl string, con *MyConn) ([]Col, error) {
	tmp, err := con.Columns(tbl)
	if err != nil {
		return nil, err
	}
	col := make([]Col, 0, len(tmp))
	for _, v := range tmp {
		col = append(col, v)
	}

	sort.Slice(col, func(i, j int) bool {
		return col[i].Seq < col[j].Seq
	})
	return col, err
}

func makeTrgList(tbl string, con *MyConn) ([]string, error) {
	trg, err := con.Triggers(tbl)
	if err != nil {
		return nil, err
	}

	rst := make([]string, 0, len(trg))
	for k := range trg {
		rst = append(rst, k)
	}
	sort.Strings(rst)
	return rst, nil
}
