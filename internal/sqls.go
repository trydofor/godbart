package internal

import (
	"fmt"
	"strings"
)

const (
	Joiner = "\n"
	SegCmt = 0
	SegRow = 1
	SegExe = 2
)

type Sql struct {
	Line string // 开始和结束行，全闭区间
	Head int    // 首行
	Type int    // 0:注释, 1:SELECT, 2:执行语句
	File string // 文件名或名字
	Text string // 正文部分
}

func ParseSqls(pref *Preference, file *FileEntity) (sqls []Sql, err error) {

	lines := splitLinex(file.Text)
	sbgn, mbgn, tbgn := -1, -1, -1

	llen := len(lines)
	sqls = make([]Sql, 0, 32)
	dt, dc := pref.DelimiterRaw, pref.DelimiterCmd

	for i, line := range lines {

		//多行注释开始
		if isCommentBgn(pref, line) {
			parseStatement(&sqls, lines, file.Path, &tbgn, i-1, &dt, dc)
			mbgn = i
			continue
		}

		// 多行注释结束
		if mbgn >= 0 {
			if isCommentEnd(pref, line) {
				parseComment(&sqls, lines, file.Path, &mbgn, i)
			}
			continue
		}

		// 单行注释开始
		if isCommentLine(pref, line) {
			parseStatement(&sqls, lines, file.Path, &tbgn, i-1, &dt, dc)
			if sbgn < 0 {
				sbgn = i
			}
			continue
		}

		// 单行注释结束
		if sbgn >= 0 {
			parseComment(&sqls, lines, file.Path, &sbgn, i-1)
		}

		e := len(line) == 0

		// SQL正文
		if tbgn < 0 && !e {
			tbgn = i
		}

		// 空行分组
		if e {
			parseStatement(&sqls, lines, file.Path, &tbgn, i-1, &dt, dc)
		}
	}

	l := llen - 1
	if sbgn > 0 {
		parseComment(&sqls, lines, file.Path, &sbgn, l)
	}
	if mbgn > 0 {
		parseComment(&sqls, lines, file.Path, &mbgn, l)
	}
	if tbgn > 0 {
		parseStatement(&sqls, lines, file.Path, &tbgn, l, &dt, dc)
	}

	return
}

func parseComment(segs *[]Sql, lines []string, name string, b *int, e int) {
	if *b < 0 || *b > e {
		return
	}

	i := e + 1
	text := strings.Join(lines[*b:i], Joiner)
	head := *b + 1
	*segs = append(*segs, Sql{
		fmt.Sprintf("%d:%d", head, i), head, SegCmt, name, text,
	})

	*b = -1
}

func parseStatement(segs *[]Sql, lines []string, name string, b *int, e int, dt *string, dc string) {
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
	//seg := func(s, e int, lines []string) Sql {
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
					head := lns + 1
					*segs = append(*segs, Sql{
						fmt.Sprintf("%d:%d", head, i),
						head,
						typ(lines[lns]),
						name,
						strings.Join(lines[lns:i], Joiner),
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
			head := lns + 1
			*segs = append(*segs, Sql{
				fmt.Sprintf("%d:%d", head, n),
				head,
				typ(lines[lns]),
				name,
				strings.Join(lines[lns:n], Joiner),
			})
			lns = n
			// fmt.Printf("\t\tget the delimitor at line %d\n", n)
		}
	}

	if lns < lne {
		head := lns + 1
		*segs = append(*segs, Sql{
			fmt.Sprintf("%d:%d", head, lne),
			head,
			typ(lines[lns]),
			name,
			strings.Join(lines[lns:lne], Joiner),
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
