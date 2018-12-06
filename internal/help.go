package internal

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

func BuiltinEnvs(envs map[string]string) {

	if _, ok := envs["USER"]; !ok {
		cu, err := user.Current()
		if err == nil {
			envs["USER"] = cu.Username
			log.Printf("[TRACE] put builtin env, k=USER, v=%q\n", cu.Username)
		} else {
			envs["USER"] = ""
			log.Fatalf("[ERROR] can NOT put builtin env, k=USER, err=%v\n", err)
		}

	}

	if _, ok := envs["HOST"]; !ok {
		ht, err := os.Hostname()
		if err == nil {
			envs["HOST"] = ht
			log.Printf("[TRACE] put builtin env, k=HOST, v=%q\n", ht)
		} else {
			envs["HOST"] = "localhost"
			log.Fatalf("[ERROR] can NOT put builtin env, k=HOST, err=%v\n", err)
		}

	}

	if _, ok := envs["DATE"]; !ok {
		dt := time.Now().Format("2006-01-02 15:04:05") // :-P
		envs["DATE"] = dt
		log.Printf("[TRACE] put builtin env, k=DATE, v=%q\n", dt)
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
