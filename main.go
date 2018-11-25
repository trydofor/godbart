package main

import (
	my "github.com/trydofor/godbart/internal"
	"github.com/urfave/cli"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func checkConf(ctx *cli.Context) *my.Config {
	file := ctx.String("c")
	log.Printf("[TRACE] got conf=%s\n", file)

	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalf("[ERROR] can read config=%s\n", file)
		os.Exit(-1)
	}

	conf, err := my.ParseToml(string(data))
	if err != nil {
		log.Fatalf("[ERROR] can not parse TOML, config=%s\n", file)
		os.Exit(-1)
	}
	return conf
}

func checkDest(ctx *cli.Context, cnf *my.Config) []my.DataSource {
	flag := ctx.StringSlice("d")
	if len(flag) == 0 {
		log.Fatal("[ERROR] no dest db selected\n")
		os.Exit(-2)
	}

	dest := make([]my.DataSource, len(flag))
	for i := 0; i < len(flag); i++ {
		if d, ok := cnf.DataSource[flag[i]]; ok {
			log.Printf("[TRACE] got dest db=%s\n", flag[i])
			dest[i] = d
		} else {
			log.Fatalf("[ERROR] db not found, dest=%s\n", flag[i])
			os.Exit(-2)
		}
	}

	return dest
}

func checkSqls(ctx *cli.Context) (files []my.FileEntity) {
	flag := ctx.StringSlice("x")
	sufx := []string{}
	for _, v := range flag {
		if len(v) > 0 {
			sufx = append(sufx, strings.ToLower(v))
		}
	}

	if ctx.NArg() == 0 {
		log.Fatal("[ERROR] must give a path or file for args\n")
		os.Exit(-3)
	}

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
			files = append(files, my.FileEntity{p, string(data)})
		}

		return nil
	}

	for _, p := range ctx.Args() {
		err := filepath.Walk(p, ff)
		if err != nil {
			log.Fatalf("[ERROR] failed to read path or file=%s\n", p)
			os.Exit(-3)
		}
	}

	if len(files) < 1 {
		log.Fatal("[ERROR] can not find any SQLs\n")
		os.Exit(-3)
	}

	return
}

func checkSrce(ctx *cli.Context, cnf *my.Config) my.DataSource {
	flag := ctx.String("s")
	if flag == "" {
		log.Fatal("[ERROR] no source db selected\n")
		os.Exit(-5)
	}

	ds, ok := cnf.DataSource[flag]
	if ok {
		log.Printf("[TRACE] got source db=%s\n", flag)
	} else {
		log.Fatalf("[ERROR] db not found in config, source=%s\n", flag)
		os.Exit(-5)
	}

	return ds
}

func checkEnvs(ctx *cli.Context) map[string]string {
	flag := ctx.StringSlice("e")

	envs := make(map[string]string)
	for _, e := range flag {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) == 2 {
			envs[kv[0]] = kv[1]
			log.Printf("[TRACE] got input env, k=%q, v=%q\n", kv[0], kv[1])
		} else {
			ov := os.Getenv(kv[0])
			if ov == "" {
				log.Fatalf("[ERROR] bad env=%q\n", e)
				os.Exit(-6)
			} else {
				log.Printf("[TRACE] got system env, k=%q, v=%q\n", kv[0], ov)
			}
		}
	}

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

	return envs
}

func checkKind(ctx *cli.Context) string {
	flag := ctx.String("k")
	kind, ok := my.TbName, false
	for _, v := range my.DiffKinds {
		if strings.EqualFold(flag, v) {
			kind = v
			ok = true
			break
		}
	}
	if !ok {
		log.Fatalf("[ERROR] unsupported (K)ind=%q\n", flag)
		os.Exit(-6)
	}
	return kind
}

func checkRegx(ctx *cli.Context) []*regexp.Regexp {
	regx := []*regexp.Regexp{}
	for _, v := range ctx.Args() {
		re, err := regexp.Compile(v)
		if err != nil {
			log.Fatalf("[ERROR] failed to compile Regexp %s, %v\n", v, err)
			os.Exit(-6)
		}
		log.Printf("[TRACE] got table regexp=%s\n", v)
		regx = append(regx, re)
	}
	return regx
}

// command //
func exec(ctx *cli.Context) (err error) {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	dest := checkDest(ctx, conf)
	test := ctx.Bool("t")
	sqls := checkSqls(ctx)
	return my.Exec(&conf.Preference, dest, sqls, test)
}

func revi(ctx *cli.Context) (err error) {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	dest := checkDest(ctx, conf)
	revi := ctx.String("r")
	test := ctx.Bool("t")
	sqls := checkSqls(ctx)
	return my.Revi(&conf.Preference, dest, sqls, revi, test)
}

func diff(ctx *cli.Context) error {
	conf := checkConf(ctx)
	srce := checkSrce(ctx, conf)
	dest := checkDest(ctx, conf)

	tbls := checkRegx(ctx)

	kind := checkKind(ctx)
	log.Printf("[TRACE] got kind=%s\n", kind)

	return my.Diff(&conf.Preference, dest, &srce, tbls, kind)
}

func move(ctx *cli.Context) error {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	srce := checkSrce(ctx, conf)
	dest := checkDest(ctx, conf)
	test := ctx.Bool("t")
	sqls := checkSqls(ctx)
	return my.Move(&conf.Preference, dest, &srce, sqls, test)
}

// cli //
func main() {

	app := cli.NewApp()

	app.Author = "github.com/trydofor"
	app.Version = "0.9.1"
	app.Compiled = time.Now()

	app.Name = "godbart"
	app.Usage = app.Name + " command args"
	app.Description = "SQL-based CLI for RDBMS schema versioning & data migration"

	//
	confFlag := &cli.StringFlag{
		Name:  "c",
		Usage: "the main (C)onfig",
		Value: "godbart.toml",
	}

	destFlag := &cli.StringSliceFlag{
		Name:  "d",
		Usage: "the (D)estination db in config",
	}

	envsFlag := &cli.StringSliceFlag{
		Name:  "e",
		Usage: "the (E)nvironment. eg. \"-e MY_DATE='2015-11-18 12:34:56'\"",
	}

	kindFlag := &cli.StringFlag{
		Name:  "k",
		Usage: "the (K)ind to diff [detail|create|tbname]. detail:table details (column, index, trigger). create:show create ddl (table, trigger). tbname:only table's name.",
		Value: "tbname",
	}

	reviFlag := &cli.StringFlag{
		Name:  "r",
		Usage: "the (R)evision to run to",
	}

	srceFlag := &cli.StringFlag{
		Name:  "s",
		Usage: "the (S)ource db in config",
	}

	testFlag := &cli.BoolFlag{
		Name:  "t",
		Usage: "only (T)est Report NOT really run",
	}

	sufxFlag := &cli.StringSliceFlag{
		Name:  "x",
		Usage: "the Suffi(X) of SQL files. eg \".sql\"",
	}

	//
	app.Commands = []cli.Command{
		{
			Name:      "exec",
			Usage:     "execute SQLs on dbs",
			ArgsUsage: "some files or paths of SQLs",
			Flags: []cli.Flag{
				confFlag,
				sufxFlag,
				destFlag,
				envsFlag,
				testFlag,
			},
			Action: exec,
		},
		{
			Name:      "revi",
			Usage:     "upgrade schema by revision",
			ArgsUsage: "some files or paths of SQLs",
			Flags: []cli.Flag{
				confFlag,
				sufxFlag,
				destFlag,
				reviFlag,
				envsFlag,
				testFlag,
			},
			Action: revi,
		},
		{
			Name:      "diff",
			Usage:     "diff table, column, index, trigger",
			ArgsUsage: "tables to diff (regexp/i). empty means all",
			Flags: []cli.Flag{
				confFlag,
				srceFlag,
				destFlag,
				kindFlag,
			},
			Action: diff,
		},
		{
			Name:      "move",
			Usage:     "move data between dbs",
			ArgsUsage: "some files or paths of SQLs",
			Flags: []cli.Flag{
				confFlag,
				srceFlag,
				destFlag,
				envsFlag,
				testFlag,
			},
			Action: move,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
