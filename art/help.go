package art

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

func logDebug(m string, a ...interface{}) {
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

func errorAndLog(m string, a ...interface{}) error {
	s := fmt.Sprintf(m, a...)
	LogError("%s", s)
	return errors.New(s)
}

func openDbAndLog(db *DataSource) (conn *MyConn, err error) {
	logDebug("try to open db=%s", db.Code)
	conn = &MyConn{}
	err = conn.Open(pref, db)

	if err == nil {
		LogTrace("successfully opened db=%s", db.Code)
	} else {
		LogError("failed to open db=%s, err=%v", db.Code, err)
	}

	return
}

// public
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

func fmtTime(t time.Time, f string) string {
	if len(f) == 0 {
		return t.Format("2006-01-02 15:04:05.000")
	} else {
		return t.Format(f)
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
		dt := time.Now().Format("2006-01-02 15:04:05") // :-P
		envs[EnvDate] = dt
		LogTrace("put builtin env, k=%s, v=%q", EnvDate, dt)
	}

	if rl, ok := envs[EnvRule]; !ok {
		envs[EnvRule] = EnvRuleError
		LogTrace("put builtin env, k=%s, v=%q", EnvRule, EnvRuleError)
	} else {
		switch rl {
		case EnvRuleEmpty, EnvRuleError:
			LogTrace("use builtin env, k=%s, v=%q", EnvRule, rl)
		default:
			ExitIfTrue(true, -4, "unsupport env, key=%s, value=%s", EnvRule, rl)
		}
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
