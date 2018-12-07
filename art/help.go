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

func errorAndLog(m string, a ...interface{}) error {
	s := fmt.Sprintf(m, a...)
	log.Fatalf("[ERROR] %s\n", s)
	return errors.New(s)
}

func openDbAndLog(db *DataSource) (conn *MyConn, err error) {
	log.Printf("[TRACE] try to open db=%s\n", db.Code)
	conn = &MyConn{}
	err = conn.Open(pref, db)

	if err == nil {
		log.Printf("[TRACE] successfully opened db=%s\n", db.Code)
	} else {
		log.Fatalf("[ERROR] failed to open db=%s, err=%v\n", db.Code, err)
	}

	return
}

// public
func ExitIfError(err error, code int, format string, args ...interface{}) {
	if err != nil {
		args = append(args, err)
		log.Fatalf("[ERROR] "+format+", err=%v\n", args...)
		os.Exit(code)
	}
}

func ExitIfTrue(tru bool, code int, format string, args ...interface{}) {
	if tru {
		log.Fatalf("[ERROR] "+format+"\n", args...)
		os.Exit(code)
	}
}

const (
	EnvUser      = "USER"
	EnvHost      = "HOST"
	EnvDate      = "DATE"
	EnvRule      = "ENV-CHECK-RULE"
	EnvRuleError = "ERROR"
	EnvRuleEmpty = "EMPTY"
)

func BuiltinEnvs(envs map[string]string) {

	if _, ok := envs[EnvUser]; !ok {
		cu, err := user.Current()
		if err == nil {
			envs[EnvUser] = cu.Username
			log.Printf("[TRACE] put builtin env, k=%s, v=%q\n", EnvUser, cu.Username)
		} else {
			envs[EnvUser] = ""
			log.Fatalf("[ERROR] put builtin env empty, k=%s, err=%v\n", EnvUser, err)
		}
	}

	if _, ok := envs[EnvHost]; !ok {
		ht, err := os.Hostname()
		if err == nil {
			envs[EnvHost] = ht
			log.Printf("[TRACE] put builtin env, k=%s, v=%q\n", EnvHost, ht)
		} else {
			envs[EnvHost] = "localhost"
			log.Fatalf("[ERROR] put builtin 'localhost', k=%s, err=%v\n", EnvHost, err)
		}
	}

	if _, ok := envs[EnvDate]; !ok {
		dt := time.Now().Format("2006-01-02 15:04:05") // :-P
		envs[EnvDate] = dt
		log.Printf("[TRACE] put builtin env, k=%s, v=%q\n", EnvDate, dt)
	}

	if rl, ok := envs[EnvRule]; !ok {
		envs[EnvRule] = EnvRuleError
		log.Printf("[TRACE] put builtin env, k=%s, v=%q\n", EnvRule, EnvRuleError)
	} else {
		switch rl {
		case EnvRuleEmpty, EnvRuleError:
			log.Printf("[TRACE] use builtin env, k=%s, v=%q\n", EnvRule, rl)
		default:
			ExitIfTrue(true, -4, "unsupport env, key=%s, value=%s", EnvRule, rl)
		}
	}
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
			log.Fatalf("[ERROR] error=%v at a path=%q\n", e, p)
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
				log.Fatalf("[ERROR] can read file=%s\n", f)
				return err
			}
			log.Printf("[TRACE] got file=%s\n", p)
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
