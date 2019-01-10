package art

import (
	"regexp"
)

var whiteRegexp = regexp.MustCompile("[ \t\r\n]+")

func removeWhite(str string) string {
	return whiteRegexp.ReplaceAllString(str, "")
}

var blankRegexp = regexp.MustCompile("[ \t]+")

func squashBlank(str string) string {
	return blankRegexp.ReplaceAllString(str, " ")
}

var trimxRegexp = regexp.MustCompile("[ \t]*[\r\n]+[ \t]*")

func squashTrimx(str string) string {
	return trimxRegexp.ReplaceAllString(str, "\n")
}

var crlfReg = regexp.MustCompile("[ \t]*(\r\n|\r|\n)[ \t]*") // 换行分割并去掉左右空白
func splitLinex(str string) []string {
	return crlfReg.Split(str, -1)
}

func countQuotePair(str string) (cnt int) {
	l := len(str)
	if l < 2 {
		return 0
	}

	cnt = 0
	for {
		i := len(str) - 1
		c, e := str[0], str[i]
		if c == e && (c == '"' || c == '\'' || c == '`') {
			cnt++
			str = str[1:i]
		} else {
			break
		}
	}
	return
}

func matchEntire(reg *regexp.Regexp, str string) bool {
	ps := reg.FindStringIndex(str)
	if len(ps) != 2 {
		return false
	}
	return ps[0] == 0 && ps[1] == len(str)
}
