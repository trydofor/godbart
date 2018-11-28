package internal

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	SegCmt = 0
	SegRow = 1
	SegExe = 2
)

type SqlSeg struct {
	Segs []Seg // 文件中所有段（SQL和注释）
	Args []Arg // 所有注释中的参数部分
}

type Arg struct {
	Line string // 开始和结束行，全闭区间
	Type string // 参数类型 ENV|REF|RUN
	Para string // 变量名
	Hold string // 占位符
}

type Seg struct {
	Line string // 开始和结束行，全闭区间
	Type int    // 0:注释, 1:SELECT, 2:执行语句
	File string // 文件名或名字
	Text string // 正文部分
}

func ParseSqls(pref *Preference, file *FileEntity) (sqls *SqlSeg, err error) {

	lines := splitLinex(file.Text)
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
				doParaArg(&args, lines, mbgn, i)
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
			doParaArg(&args, lines, sbgn, i-1)
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
		doParaArg(&args, lines, sbgn, l)
		doComment(&segs, lines, file.Path, &sbgn, l)
	}
	if mbgn > 0 {
		doParaArg(&args, lines, mbgn, l)
		doComment(&segs, lines, file.Path, &mbgn, l)
	}
	if tbgn > 0 {
		doSqlSeg(&segs, lines, file.Path, &tbgn, l, &dt, dc)
	}

	sqls = &SqlSeg{segs, args}
	return
}

const (
	CmndEnv = "ENV"
	CmndRef = "REF"
	CmndStr = "STR"
	CmndRun = "RUN"
	CmndOut = "OUT"
)

var cmdArrs = []string{CmndEnv, CmndRef, CmndStr, CmndRun, CmndOut}

var argsReg = regexp.MustCompile(`(?i)` + // 不区分大小写
	`^[^0-9A-Z]*` + // 非英数开头，视为注释部分
	`(` + strings.Join(cmdArrs, "|") + `)[ \t]+` + //命令和空白，第一分组，固定值
	"([^`'\" \t]+|'[^']+'|\"[^\"]+\"|`[^`]+`)[ \t]+" + // 变量和空白，第二分组，
	"([^`'\" \t]+|'[^']+'|\"[^\"]+\"|`[^`]+`)") // 连续的非引号空白或，引号成对括起来的字符串（贪婪）

func doParaArg(args *[]Arg, lines []string, b int, e int) {
	// 分析参数 ENV REF RUN
	ln := fmt.Sprintf("%d:%d", b+1, e+1)

	for _, v := range lines[b : e+1] {
		sm := argsReg.FindStringSubmatch(v)
		if len(sm) == 4 {
			cmd := strings.ToUpper(sm[1])
			if cmd == CmndRun || cmd == CmndOut {
				sm[2] = strings.ToUpper(sm[2]) // 命令变量大写
			} else if cmd == CmndRef || cmd == CmndEnv {
				if cp := countQuotePair(sm[2]); cp > 0 { //引用变量脱引号
					sm[2] = sm[2][cp : len(sm[2])-cp]
				}
			}

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
		fmt.Sprintf("%d:%d", *b+1, i), SegCmt, name, text,
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
			return SegRow
		} else {
			return SegExe
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

// helper

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
