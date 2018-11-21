package internal

import (
	"fmt"
	"regexp"
	"strings"
)

type SqlDyn struct {
	Segs []Seg // 文件中所有段（SQL和注释）
	Args []Arg // 所有注释中的参数部分
	Exes []Exe // SQL执行时的依赖
}

type Exe struct {
	Seg  *Seg              // 对应的SQL片段
	Stm  string            // 执行的SQL statement,`?`替换
	Args []*Arg            // statement 需要的参数，`？`的顺序
	Envs map[string]string // 依赖的ENV
	Deps map[string]string // 依赖的REF
	Refs map[string]string // 产生的REF
	Fork []*Exe            // 结束时分叉执行
	Done []*Exe            // 结束时执行的RUN
}

type Arg struct {
	Line string // 开始和结束行，全闭区间
	Type string // 参数类型 ENV|REF|RUN
	Para string // 变量名
	Hold string // 占位符
}

const (
	COMMENT = 0
	SELECTS = 1
	EXECUTE = 2
)

type Seg struct {
	Line string // 开始和结束行，全闭区间
	Type int    // 是否为注释0，SELECT1，
	File string // 文件名或名字
	Text string // 正文部分
}

var crlfReg = regexp.MustCompile(`[ \t]*(\r\n|\r|\n)[ \t]*`) // 换行分割并去掉左右空白
var argsReg = regexp.MustCompile(`(?i)` +                    // 不区分大小写
	`^[^0-9A-Z]*` + // 非英数开头，一般为注释
	`(ENV|REF|RUN|STR)[ \t]+` + //命令和空白，第一分组，固定值
	"([^`'\" \t]+|'.+'|\".+\"|`.+`)[ \t]+" + // 变量和空白，第二分组，英数下划线
	"([^`'\" \t]+|'.+'|\".+\"|`.+`)") // 连续非引号空白或，单双反引号成对括起来的字符串（贪婪）

func ParseSql(pref *Preference, file *FileEntity) (sqld *SqlDyn, err error) {

	lines := crlfReg.Split(file.Text, -1)
	sbgn, mbgn, tbgn := -1, -1, -1

	segs := []Seg{}
	args := []Arg{}
	dt, dc := pref.DelimiterRaw, pref.DelimiterCmd

	for i, line := range lines {

		//多行注释开始
		if isCommentBgn(pref, line) {
			doSqlSeg(&segs, lines, file.Path, &tbgn, i-1, &dt, dc)
			mbgn = i
			continue
		}

		// 多行注释结束
		if mbgn >= 0 {
			if isCommentEnd(pref, line) {
				doCmntArg(&args, lines, mbgn, i)
				doComment(&segs, lines, file.Path, &mbgn, i)
			}
			continue
		}

		// 单行注释开始
		if isCommentLine(pref, line) {
			doSqlSeg(&segs, lines, file.Path, &tbgn, i-1, &dt, dc)
			if sbgn < 0 {
				sbgn = i
			}
			continue
		}

		// 单行注释结束
		if sbgn >= 0 {
			doCmntArg(&args, lines, sbgn, i-1)
			doComment(&segs, lines, file.Path, &sbgn, i-1)
		}

		e := len(line) == 0

		// SQL正文
		if tbgn < 0 && !e {
			tbgn = i
		}

		// 空行分组
		if e {
			doSqlSeg(&segs, lines, file.Path, &tbgn, i-1, &dt, dc)
		}
	}

	l := len(lines) - 1
	if sbgn > 0 {
		doCmntArg(&args, lines, sbgn, l)
		doComment(&segs, lines, file.Path, &sbgn, l)
	}
	if mbgn > 0 {
		doCmntArg(&args, lines, mbgn, l)
		doComment(&segs, lines, file.Path, &mbgn, l)
	}
	if tbgn > 0 {
		doSqlSeg(&segs, lines, file.Path, &tbgn, l, &dt, dc)
	}

	exes := doExeTree(pref, segs, args)
	sqld = &SqlDyn{segs, args, exes}
	return
}

func doExeTree(pref *Preference, segs []Seg, args []Arg) (exes []Exe) {
	// 大部分情况，直接返回
	if len(args) == 0 {
		for _, v := range segs {
			if v.Type != COMMENT {
				exes = append(exes, Exe{&v, v.Text, nil, nil, nil, nil, nil, nil})
			}
		}
		return
	}

	//deep := make(map[string]int) // 某行处SQL段的深度

	for i, v := range segs {
		if v.Type != COMMENT {
			// TODO
			fmt.Printf("\ttodo %d\n", i)
		}
	}
	return
}

func doCmntArg(args *[]Arg, lines []string, b int, e int) {
	// 分析参数 ENV REF RUN
	ln := fmt.Sprintf("%d:%d", b+1, e+1)
	var tm = func(r rune) bool {
		return r == '"' || r == '\'' || r == '`'
	}

	for _, v := range lines[b : e+1] {
		sm := argsReg.FindStringSubmatch(v)
		if len(sm) == 4 {
			cmd := strings.ToUpper(sm[1])
			if cmd == "RUN" {
				sm[2] = strings.ToUpper(sm[2]) // 条件命令大写
			} else {
				sm[2] = strings.TrimFunc(sm[2], tm) // 变量去掉引号
			}

			//if cmd == "ENV" {
			//	sm[3] = strings.TrimFunc(sm[3], tm) // 环境变量的占位去引号
			//}

			arg := Arg{ln, cmd, sm[2], sm[3]}
			*args = append(*args, arg)
		}
	}
}

func doComment(segs *[]Seg, lines []string, name string, b *int, e int) {
	if *b < 0 || *b > e {
		return
	}

	i := e + 1
	text := strings.Join(lines[*b:i], "\n")
	*segs = append(*segs, Seg{
		fmt.Sprintf("%d:%d", *b+1, i), COMMENT, name, text,
	})

	*b = -1
}

func doSqlSeg(segs *[]Seg, lines []string, name string, b *int, e int, dt *string, dc string) {
	if *b < 0 || *b > e {
		return
	}

	// 处理结束符
	lns, lne := *b, e+1
	dtl, dcl := len(*dt), len(dc)
	typ := func(sql string) int {
		if strings.EqualFold("SELECT", sql[0:6]) {
			return SELECTS
		} else {
			return EXECUTE
		}
	}
	//seg := func(s, e int, lines []string) Seg {
	//
	//}
	for i := lns; i < lne; i++ {
		l := len(lines[i])
		n := i + 1
		if dcl > 0 && l > dcl && strings.EqualFold(dc, lines[i][0:dcl]) { // 变更结束符
			c := lines[i][dcl]
			if c == ' ' || c == '\t' {
				*dt = strings.TrimSpace(lines[i][dcl+1:])
				dtl = len(*dt)
				if lns < i { // 结束上一段
					*segs = append(*segs, Seg{
						fmt.Sprintf("%d:%d", lns+1, i),
						typ(lines[lns]),
						name,
						strings.Join(lines[lns:i], "\n"),
					})
				}
				lns = n
				// fmt.Printf("\t\tget new delimitor [%s] at line %d\n", *dt, n)
				continue
			}
		}

		dtp := l - dtl
		if dtl > 0 && l > dtl && strings.EqualFold(*dt, lines[i][dtp:]) { // 结束符
			lines[i] = lines[i][0:dtp]
			*segs = append(*segs, Seg{
				fmt.Sprintf("%d:%d", lns+1, n),
				typ(lines[lns]),
				name,
				strings.Join(lines[lns:n], "\n"),
			})
			lns = n
			// fmt.Printf("\t\tget the delimitor at line %d\n", n)
		}
	}

	if lns < lne {
		*segs = append(*segs, Seg{
			fmt.Sprintf("%d:%d", lns+1, lne),
			typ(lines[lns]),
			name,
			strings.Join(lines[lns:lne], "\n"),
		})
	}
	*b = -1
}

func isCommentLine(pref *Preference, str string) bool {
	if pref.LineComment == "" {
		return false
	}
	return strings.HasPrefix(str, pref.LineComment)
}

func isCommentBgn(pref *Preference, str string) bool {
	l := len(pref.MultComment)
	if l < 2 {
		return false
	}

	for i := 0; i < l; i += 2 {
		if strings.HasPrefix(str, pref.MultComment[i]) {
			return true
		}
	}
	return false
}

func isCommentEnd(pref *Preference, str string) bool {
	l := len(pref.MultComment)
	if l < 2 {
		return false
	}

	for i := 1; i < l; i += 2 {
		if strings.HasSuffix(str, pref.MultComment[i]) {
			return true
		}
	}

	return false
}
