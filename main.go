package main

import (
	"fmt"
	"github.com/trydofor/godbart/art"
	"github.com/urfave/cli"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

func checkConf(ctx *cli.Context) *art.Config {
	file := ctx.String("c")
	log.Printf("[TRACE] got conf=%s\n", file)

	data, err := ioutil.ReadFile(file)
	art.ExitIfError(err, -1, "can read config=%s", file)

	conf, err := art.ParseToml(string(data))
	art.ExitIfError(err, -1, "can not parse TOML, config=%s", file)

	return conf
}

func checkDest(ctx *cli.Context, cnf *art.Config, req bool) []*art.DataSource {
	flag := ctx.StringSlice("d")
	art.ExitIfTrue(req && len(flag) == 0, -2, "no dest db selected")

	dest := make([]*art.DataSource, len(flag))
	for i := 0; i < len(flag); i++ {
		d, ok := cnf.DataSource[flag[i]]
		art.ExitIfTrue(!ok, -2, "db not found, dest=%s", flag[i])
		log.Printf("[TRACE] got dest db=%s\n", flag[i])
		dest[i] = &d
	}

	return dest
}

func checkSrce(ctx *cli.Context, cnf *art.Config, req bool) *art.DataSource {
	flag := ctx.String("s")
	art.ExitIfTrue(req && len(flag) == 0, -5, "no source db selected")

	ds, ok := cnf.DataSource[flag]
	art.ExitIfTrue(!ok, -5, "db not found in config, source=%s", flag)
	log.Printf("[TRACE] got source db=%s\n", flag)

	return &ds
}

func checkSqls(ctx *cli.Context) (files []art.FileEntity) {
	art.ExitIfTrue(ctx.NArg() == 0, -3, "must give a path or file for args")

	flag := ctx.StringSlice("x")
	files, err := art.FileWalker(ctx.Args(), flag)
	art.ExitIfError(err, -3, "failed to read file")
	art.ExitIfTrue(len(files) < 1, -3, "can not find any SQLs")

	return
}

func checkEnvs(ctx *cli.Context) map[string]string {
	flag := ctx.StringSlice("e")

	envs := make(map[string]string)
	for _, env := range flag {
		kv := strings.SplitN(env, "=", 2)
		if len(kv) == 2 {
			envs[kv[0]] = kv[1]
			log.Printf("[TRACE] got input env, k=%q, v=%q\n", kv[0], kv[1])
		} else {
			ov, ok := os.LookupEnv(kv[0])
			art.ExitIfTrue(!ok, -6, "system ENV not found, env=%q", env)
			log.Printf("[TRACE] got system env, k=%q, v=%q\n", kv[0], ov)
		}
	}

	art.BuiltinEnvs(envs)

	return envs
}

func checkKind(ctx *cli.Context) string {
	flag := ctx.String("k")
	kind, ok := art.TbName, false
	for _, v := range art.DiffKinds {
		if strings.EqualFold(flag, v) {
			kind = v
			ok = true
			break
		}
	}
	art.ExitIfTrue(!ok, -6, "unsupported (K)ind=%q", flag)
	log.Printf("[TRACE] got kind=%s\n", flag)
	return kind
}

func checkRegx(ctx *cli.Context) []*regexp.Regexp {
	args := ctx.Args()
	regx := make([]*regexp.Regexp, 0, len(args))
	for _, v := range args {
		re, err := regexp.Compile(v)
		art.ExitIfError(err, -6, "failed to compile Regexp=%v", v)
		log.Printf("[TRACE] got table regexp=%s\n", v)
		regx = append(regx, re)
	}
	return regx
}

func checkRisk(ctx *cli.Context) bool {
	agr := ctx.Bool("agree")
	return agr
}

// command //
func exec(ctx *cli.Context) (err error) {
	conf := checkConf(ctx)
	dest := checkDest(ctx, conf, true)
	risk := checkRisk(ctx)
	sqls := checkSqls(ctx)
	return art.Exec(&conf.Preference, dest, sqls, risk)
}

func revi(ctx *cli.Context) (err error) {
	conf := checkConf(ctx)
	dest := checkDest(ctx, conf, true)
	revi := ctx.String("r")
	mask := ctx.String("m")
	rqry := ctx.String("q")
	risk := checkRisk(ctx)
	sqls := checkSqls(ctx)
	return art.Revi(&conf.Preference, dest, sqls, revi, mask, rqry, risk)
}

func diff(ctx *cli.Context) error {
	conf := checkConf(ctx)
	dest := checkDest(ctx, conf, false)
	srce := checkSrce(ctx, conf, false)
	kind := checkKind(ctx)
	tbls := checkRegx(ctx)
	return art.Diff(&conf.Preference, srce, dest, kind, tbls)
}

func tree(ctx *cli.Context) error {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	srce := checkSrce(ctx, conf, true)
	dest := checkDest(ctx, conf, false)
	risk := checkRisk(ctx)
	sqls := checkSqls(ctx)
	return art.Tree(&conf.Preference, conf.StartupEnv, srce, dest, sqls, risk)
}

func sqlx(ctx *cli.Context) error {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	sqls := checkSqls(ctx)
	sqlx, err := art.ParseTree(&conf.Preference, conf.StartupEnv, sqls)
	if err != nil {
		return err
	}

	for i, t := range sqlx {
		fmt.Printf("\n==== envx file=%s ====", sqls[i].Path)
		for k, v := range t.Envs {
			fmt.Printf("\n%s=%s", k, v)
		}

		fmt.Printf("\n==== exex file=%s ====", sqls[i].Path)
		for _, x := range t.Exes {
			fmt.Printf("\n%v", x)
		}
	}
	return nil
}

// cli //
func main() {

	app := cli.NewApp()

	app.Author = "github.com/trydofor"
	app.Version = "0.9.2"
	app.Compiled = time.Now()

	app.Name = "godbart"
	app.Usage = app.Name + " command args"
	app.Description = `SQL-based CLI for RDBMS schema versioning & data migration

		readme   - https://github.com/trydofor/godbart
		config   - https://github.com/trydofor/godbart/blob/master/godbart.toml
		demo sql - https://github.com/trydofor/godbart/tree/master/demo/sql/
`

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

	maskFlag := &cli.StringFlag{
		Name:  "m",
		Usage: "the (M)ask (regexp) of the revision",
		Value: "[0-9]{10,}",
	}

	rqryFlag := &cli.StringFlag{
		Name:  "q",
		Usage: "the (Q)uery Prefix (string) of revision",
		Value: "SELECT",
	}

	reviFlag := &cli.StringFlag{
		Name:  "r",
		Usage: "the (R)evision to run to",
	}

	srceFlag := &cli.StringFlag{
		Name:  "s",
		Usage: "the (S)ource db in config",
	}

	riskFlag := &cli.BoolFlag{
		Name:  "agree",
		Usage: "dangerous SQL can lost data, you agree to take any risk on yourself!",
	}

	sufxFlag := &cli.StringSliceFlag{
		Name:  "x",
		Usage: "the Suffi(X) (string) of SQL files. eg \".sql\"",
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
				riskFlag,
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
				maskFlag,
				rqryFlag,
				riskFlag,
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
			Name:      "tree",
			Usage:     "move tree data between dbs",
			ArgsUsage: "some files or paths of SQLs",
			Flags: []cli.Flag{
				confFlag,
				sufxFlag,
				destFlag,
				srceFlag,
				envsFlag,
				riskFlag,
			},
			Action: tree,
		},
		{
			Name:      "sqlx",
			Usage:     "static analyze data-tree by sql file",
			ArgsUsage: "some files or paths of SQLs",
			Flags: []cli.Flag{
				confFlag,
				envsFlag,
			},
			Action: sqlx,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
