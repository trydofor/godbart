package art

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// public

func LogDebug(m string, a ...interface{}) {
	if MsgLevel >= LvlDebug {
		log.Printf("[DEBUG] "+m+"\n", a...)
	}
}

func LogTrace(m string, a ...interface{}) {
	if MsgLevel >= LvlTrace {
		log.Printf("[TRACE] "+m+"\n", a...)
	}

}

func LogError(m string, a ...interface{}) {
	if MsgLevel >= LvlError {
		log.Printf("[ERROR] "+m+"\n", a...)
	}
}

func LogFatal(m string, a ...interface{}) {
	if MsgLevel >= LvlError {
		log.Fatalf("[FATAL] "+m+"\n", a...)
	}
}

func OutDebug(m string, a ...interface{}) {
	if MsgLevel >= LvlDebug {
		fmt.Printf(m+"\n", a...)
	}
}

func OutTrace(m string, a ...interface{}) {
	fmt.Printf(m+"\n", a...)
}

func ExitIfError(err error, code int, format string, args ...interface{}) {
	if err != nil {
		args = append(args, err)
		LogError(""+format+", err=%v", args...)
		os.Exit(code)
	}
}

func ExitIfTrue(tru bool, code int, format string, args ...interface{}) {
	if tru {
		LogError(""+format+"", args...)
		os.Exit(code)
	}
}

func BuiltinEnvs(envs map[string]string) {

	if _, ok := envs[EnvUser]; !ok {
		cu, err := user.Current()
		if err == nil {
			envs[EnvUser] = cu.Username
			LogTrace("put builtin env, k=%s, v=%q", EnvUser, cu.Username)
		} else {
			envs[EnvUser] = ""
			LogFatal("put builtin env empty, k=%s, err=%v", EnvUser, err)
		}
	}

	if _, ok := envs[EnvHost]; !ok {
		ht, err := os.Hostname()
		if err == nil {
			envs[EnvHost] = ht
			LogTrace("put builtin env, k=%s, v=%q", EnvHost, ht)
		} else {
			envs[EnvHost] = "localhost"
			LogFatal("put builtin 'localhost', k=%s, err=%v", EnvHost, err)
		}
	}

	if _, ok := envs[EnvDate]; !ok {
		dt := fmtTime(time.Now(),"2006-01-02 15:04:05") // :-P
		envs[EnvDate] = dt
		LogTrace("put builtin env, k=%s, v=%q", EnvDate, dt)
	}

	if rl, ok := envs[EnvRule]; !ok {
		LogTrace("builtin env, k=%s not found", EnvRule)
	} else {
		LogTrace("use builtin env, k=%s, v=%q", EnvRule, rl)
	}

	envs[EnvSrcDb] = "UN-SET"
	envs[EnvOutDb] = "UN-SET"
}

func FileWalker(path []string, flag []string) ([]FileEntity, error) {

	sufx := make([]string, 0, len(flag))
	for _, v := range flag {
		if len(v) > 0 {
			sufx = append(sufx, strings.ToLower(v))
		}
	}

	var files []FileEntity
	var ff = func(p string, f os.FileInfo, e error) error {

		if e != nil {
			LogError("error=%v at a path=%q", e, p)
			return e
		}

		if f.IsDir() {
			return nil
		}

		h := false
		if len(sufx) > 0 {
			l := strings.ToLower(p)
			for _, v := range sufx {
				if strings.HasSuffix(l, v) {
					h = true
					break
				}
			}
		} else {
			h = true
		}

		if h {
			data, err := ioutil.ReadFile(p)
			if err != nil {
				LogError("can read file=%s", f)
				return err
			}
			LogTrace("got file=%s", p)
			files = append(files, FileEntity{p, string(data)})
		}

		return nil
	}

	for _, p := range path {
		err := filepath.Walk(p, ff)
		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

// private
func errorAndLog(m string, a ...interface{}) error {
	s := fmt.Sprintf(m, a...)
	LogError("%s", s)
	return errors.New(s)
}

func openDbAndLog(db *DataSource) (conn *MyConn, err error) {
	LogDebug("try to open db=%s", db.Code)
	conn = &MyConn{}
	err = conn.Open(pref, db)

	if err == nil {
		LogTrace("successfully opened db=%s", db.Code)
	} else {
		LogError("failed to open db=%s, err=%v", db.Code, err)
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

	for _, r := range rgx {
		for _, t := range tbs {
			if matchEntire(r, t) {
				rst = append(rst, t)
			}
		}
	}
	return
}

func walkExes(exes []*Exe, fn func(exe *Exe) error) error {
	for _, exe := range exes {
		er := fn(exe)
		if er != nil {
			return er
		}
		er = walkExes(exe.Sons, fn)
		if er != nil {
			return er
		}
	}
	return nil
}

// 只支持 SEQ|TBL
func pureRunExes(exes []*Exe, ctx map[string]interface{}, db *MyConn, fn func(exe *Exe, stm string) error) (err error) {
	for _, exe := range exes {
		if len(exe.Fors) == 0 {
			err = pureOneExes(exe, ctx, db, fn)
		} else {
			for i, arg := range exe.Fors {
				LogDebug("FOR exe [%d] on Arg=%s, exe=%d", i+1, arg, exe.Seg.Head)
				var vals []string
				switch arg.Type {
				case CmndSeq:
					gift := arg.Gift.(GiftSeq)
					for j := gift.Bgn; j <= gift.End; j = j + gift.Inc {
						v := fmt.Sprintf(gift.Fmt, j)
						vals = append(vals, v)
						LogDebug("FOR SEQ on Arg=%d, exe=%d, seq=%s", arg.Head, exe.Seg.Head, v)
					}
				case CmndTbl:
					tblKey := arg.Hold + magicDatabaseSrcTable
					tbls, ok := ctx[tblKey]
					if !ok {
						tbls, err = db.Tables()
						if err != nil {
							return err
						}
						ctx[tblKey] = tbls
					}

					reg := arg.Gift.(*regexp.Regexp)
					for _, v := range tbls.([]string) {
						if matchEntire(reg, v) {
							vals = append(vals, v)
							LogDebug("FOR TBL on Arg=%d, exe=%d, table=%s", arg.Head, exe.Seg.Head, v)
						}
					}
				default:
					return errorAndLog("unsupported FOR arg=%s", arg)
				}

				for _, v := range vals {
					LogTrace("FOR %s on Arg=%d, exe=%d, value=%s", arg.Type, arg.Head, exe.Seg.Head, v)
					ctx[arg.Hold] = v
					err = pureOneExes(exe, ctx, db, fn)
					if err != nil {
						return
					}
				}
			}
		}
	}
	return
}

func pureOneExes(exe *Exe, ctx map[string]interface{}, db *MyConn, fn func(exe *Exe, stm string) error) error {
	stm := exe.Seg.Text
	if len(exe.Deps) > 0 {
		// build stmt
		var sb strings.Builder // return,stdout
		off := 0
		for _, dep := range exe.Deps {
			LogDebug("parsing dep=%s", dep)

			if dep.Off > off {
				tmp := stm[off:dep.Off]
				sb.WriteString(tmp)
			}

			off = dep.End
			hld := dep.Str
			if ev, ok := ctx[hld]; ok && !dep.Dyn {
				v := ev.(string)
				sb.WriteString(v)
				LogDebug("static simple replace hold=%s, with value=%s", hld, v)
			} else {
				return errorAndLog("unsupported hold=%s in pure type, exe.head=%d, file=%s", hld, exe.Seg.Head, exe.Seg.File)
			}
		}

		if off > 0 && off < len(stm) {
			sb.WriteString(stm[off:])
		}
		if off > 0 {
			stm = sb.String()
		}
	}

	err := fn(exe, stm)
	if err != nil {
		return err
	}

	pureRunExes(exe.Sons, ctx, db, fn)

	return err
}
