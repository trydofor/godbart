package main

import (
	my "github.com/trydofor/godbart/internal"
	"github.com/urfave/cli"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

func checkConf(ctx *cli.Context) *my.Config {
	file := ctx.String("c")
	log.Printf("[TRACE] got conf=%s\n", file)

	data, err := ioutil.ReadFile(file)
	my.ExitIfError(err, -1, "can read config=%s", file)

	conf, err := my.ParseToml(string(data))
	my.ExitIfError(err, -1, "can not parse TOML, config=%s", file)

	return conf
}

func checkDest(ctx *cli.Context, cnf *my.Config) []*my.DataSource {
	flag := ctx.StringSlice("d")
	my.ExitIfTrue(len(flag) == 0, -2, "no dest db selected")

	dest := make([]*my.DataSource, len(flag))
	for i := 0; i < len(flag); i++ {
		d, ok := cnf.DataSource[flag[i]]
		my.ExitIfTrue(!ok, -2, "db not found, dest=%s", flag[i])
		log.Printf("[TRACE] got dest db=%s\n", flag[i])
		dest[i] = &d
	}

	return dest
}

func checkSqls(ctx *cli.Context) (files []my.FileEntity) {
	my.ExitIfTrue(ctx.NArg() == 0, -3, "must give a path or file for args")

	flag := ctx.StringSlice("x")
	files, err := my.FileWalker(ctx.Args(), flag)
	my.ExitIfError(err, -3, "failed to read file")
	my.ExitIfTrue(len(files) < 1, -3, "can not find any SQLs")

	return
}

func checkSrce(ctx *cli.Context, cnf *my.Config) *my.DataSource {
	flag := ctx.String("s")
	my.ExitIfTrue(len(flag) == 0, -5, "no source db selected")

	ds, ok := cnf.DataSource[flag]
	my.ExitIfTrue(!ok, -5, "db not found in config, source=%s", flag)
	log.Printf("[TRACE] got source db=%s\n", flag)

	return &ds
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
			ov := os.Getenv(kv[0])
			my.ExitIfTrue(ov == "", -6, "bad env=%q", env)
			log.Printf("[TRACE] got system env, k=%q, v=%q\n", kv[0], ov)
		}
	}

	my.BuiltinEnvs(envs)

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
	my.ExitIfTrue(!ok, -6, "unsupported (K)ind=%q", flag)
	log.Printf("[TRACE] got kind=%s\n", flag)
	return kind
}

func checkRegx(ctx *cli.Context) []*regexp.Regexp {
	args := ctx.Args()
	regx := make([]*regexp.Regexp, 0, len(args))
	for _, v := range args {
		re, err := regexp.Compile(v)
		my.ExitIfError(err, -6, "failed to compile Regexp=%v", v)
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
	dest := checkDest(ctx, conf)
	risk := checkRisk(ctx)
	sqls := checkSqls(ctx)
	return my.Exec(&conf.Preference, dest, sqls, risk)
}

func revi(ctx *cli.Context) (err error) {
	conf := checkConf(ctx)
	dest := checkDest(ctx, conf)
	revi := ctx.String("r")
	mask := ctx.String("m")
	risk := checkRisk(ctx)
	sqls := checkSqls(ctx)
	return my.Revi(&conf.Preference, dest, sqls, revi, mask, risk)
}

func diff(ctx *cli.Context) error {
	conf := checkConf(ctx)
	dest := checkDest(ctx, conf)
	srce := checkSrce(ctx, conf)
	kind := checkKind(ctx)
	tbls := checkRegx(ctx)
	return my.Diff(srce, dest, kind, tbls)
}

func tree(ctx *cli.Context) error {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	srce := checkSrce(ctx, conf)
	dest := checkDest(ctx, conf)
	risk := checkRisk(ctx)
	sqls := checkSqls(ctx)
	return my.Tree(&conf.Preference, conf.StartupEnv, srce, dest, sqls, risk)
}

// cli //
func main() {

	app := cli.NewApp()

	app.Author = "github.com/trydofor"
	app.Version = "0.9.1"
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
		Usage: "the (M)ask of the revision",
		Value: "[0-9]{10,}",
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
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
