package main

import (
	"github.com/trydofor/godbart/art"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

func checkConf(ctx *cli.Context) *art.Config {
	file := ctx.String("c")
	art.LogTrace("got conf=%s", file)

	data, err := ioutil.ReadFile(file)
	art.ExitIfError(err, -1, "can read config=%s", file)

	conf, err := art.ParseToml(string(data))
	art.ExitIfError(err, -1, "can not parse TOML, config=%s", file)

	return conf
}

func checkMlvl(ctx *cli.Context) {
	lvl := ctx.String("l")
	art.LogTrace("got level=%s", lvl)
	switch strings.ToLower(lvl) {
	case "debug":
		art.MsgLevel = art.LvlDebug
	case "trace":
		art.MsgLevel = art.LvlTrace
	case "error":
		art.MsgLevel = art.LvlError
	default:
		art.MsgLevel = art.LvlDebug
	}
}

func checkDest(ctx *cli.Context, cnf *art.Config, req bool) []*art.DataSource {
	flag := ctx.StringSlice("d")
	art.ExitIfTrue(req && len(flag) == 0, -2, "no dest db selected")

	dest := make([]*art.DataSource, len(flag))
	for i := 0; i < len(flag); i++ {
		d, ok := cnf.DataSource[flag[i]]
		art.ExitIfTrue(!ok, -2, "db not found, dest=%s", flag[i])
		art.LogTrace("got dest db=%s", flag[i])
		dest[i] = &d
	}

	return dest
}

func checkSrce(ctx *cli.Context, cnf *art.Config, req bool) *art.DataSource {
	flag := ctx.String("s")
	art.ExitIfTrue(req && len(flag) == 0, -5, "no source db selected")

	ds, ok := cnf.DataSource[flag]
	art.ExitIfTrue(!ok, -5, "db not found in config, source=%s", flag)
	art.LogTrace("got source db=%s", flag)

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

func buildEnvs(ctx *cli.Context, envs map[string]string) {
	flag := ctx.StringSlice("e")
	for _, env := range flag {
		kv := strings.SplitN(env, "=", 2)
		if len(kv) == 2 {
			envs[kv[0]] = kv[1]
			art.LogTrace("got input env, k=%q, v=%q", kv[0], kv[1])
		} else {
			ov, ok := os.LookupEnv(kv[0])
			art.ExitIfTrue(!ok, -6, "system ENV not found, env=%q", env)
			art.LogTrace("got system env, k=%q, v=%q", kv[0], ov)
		}
	}

	art.BuiltinEnvs(envs)
	return
}

func checkTmpl(ctx *cli.Context, tmpl map[string]string) (tps []string) {
	flag := ctx.String("t")
	keys := strings.SplitN(flag, ",", -1)
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if tp, ok := tmpl[k]; ok {
			tps = append(tps, k, tp)
			art.LogTrace("got tmpl in sqltemplet, key=%s", k)
		} else {
			art.ExitIfTrue(!ok, -6, "templet not found in sqltemplet, key=%s", k)
		}
	}
	return
}

func checkType(ctx *cli.Context, knd map[string]bool, dft string) map[string]bool {
	flag := ctx.String("t")
	rst := make(map[string]bool)
	for _, k := range strings.SplitN(flag, ",", -1) {
		if knd[k] {
			rst[k] = true
			art.LogTrace("got type=%s", k)
		} else {
			art.ExitIfTrue(true, -6, "unsupported (T)ype=%s, in %s", k, flag)
		}
	}
	if len(rst) == 0 {
		rst[dft] = true
	}
	return rst
}

func checkRegx(ctx *cli.Context) []*regexp.Regexp {
	args := ctx.Args()
	regx := make([]*regexp.Regexp, 0, len(args))
	for _, v := range args {
		re, err := regexp.Compile(v)
		art.ExitIfError(err, -6, "failed to compile Regexp=%v", v)
		art.LogTrace("got table regexp=%s", v)
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
	checkMlvl(ctx)
	conf := checkConf(ctx)
	dest := checkDest(ctx, conf, true)
	risk := checkRisk(ctx)
	sqls := checkSqls(ctx)
	return art.Exec(&conf.Preference, dest, sqls, risk)
}

func revi(ctx *cli.Context) (err error) {
	checkMlvl(ctx)
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
	checkMlvl(ctx)
	conf := checkConf(ctx)
	dest := checkDest(ctx, conf, false)
	srce := checkSrce(ctx, conf, false)
	kind := checkType(ctx, art.DiffType, art.DiffSum)
	tbls := checkRegx(ctx)
	return art.Diff(srce, dest, kind, tbls)
}

func show(ctx *cli.Context) error {
	checkMlvl(ctx)
	conf := checkConf(ctx)
	srce := checkSrce(ctx, conf, false)
	ktpl := checkTmpl(ctx, conf.SqlTemplet)
	tbls := checkRegx(ctx)
	return art.Show(srce, ktpl, tbls)
}

func synk(ctx *cli.Context) error {
	checkMlvl(ctx)
	conf := checkConf(ctx)
	dest := checkDest(ctx, conf, false)
	srce := checkSrce(ctx, conf, false)
	kind := checkType(ctx, art.SyncType, art.SyncTbl)
	tbls := checkRegx(ctx)
	return art.Sync(srce, dest, kind, tbls)
}

func tree(ctx *cli.Context) error {
	checkMlvl(ctx)
	conf := checkConf(ctx)
	buildEnvs(ctx, conf.StartupEnv)
	srce := checkSrce(ctx, conf, true)
	dest := checkDest(ctx, conf, false)
	risk := checkRisk(ctx)
	sqls := checkSqls(ctx)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go art.CtrlRoom.Open(conf.Preference.ControlPort, art.CtrlRoomTree, wg)
	wg.Wait()
	return art.Tree(&conf.Preference, conf.StartupEnv, srce, dest, sqls, risk)
}

func sqlx(ctx *cli.Context) error {
	checkMlvl(ctx)
	conf := checkConf(ctx)
	buildEnvs(ctx, conf.StartupEnv)
	conf.StartupEnv["ENV-CHECK-RULE"] = "EMPTY"
	sqls := checkSqls(ctx)
	sqlx, err := art.ParseTree(&conf.Preference, conf.StartupEnv, sqls)
	if err != nil {
		return err
	}

	for i, t := range sqlx {
		pth := sqls[i].Path
		art.OutTrace("==== tree=%s ====", pth)
		for _, x := range t.Exes {
			art.OutTrace("%s", x.Tree())
		}
		art.OutTrace("==== debug to see more ====")

		art.OutDebug("==== envx file=%s ====", pth)
		for k, v := range t.Envs {
			art.OutDebug("%s=%s", k, v)
		}

		art.OutDebug("==== exex file=%s ====", pth)
		for _, x := range t.Exes {
			art.OutDebug("%v", x)
		}
	}
	return nil
}

// cli //
func main() {

	app := cli.NewApp()

	app.Author = "github.com/trydofor"
	app.Version = "0.9.9"
	app.Compiled = time.Now()

	app.Name = "godbart"
	app.Usage = "god, bart is a boy of ten."
	app.UsageText = app.Name + " command [options] [arguments...]"

	app.Description = `a SQL-based CLI for RDBMS versioning & migration

    use "command -h" to see command's help and example.
	supposing godbart in $PATH and godbart.toml in $PWD 

    opt  - require exactly one
    opt? - optional zero or one
    opt* - conditional zero or more

    readme - https://github.com/trydofor/godbart
    config - https://github.com/trydofor/godbart/blob/master/godbart.toml
    sample - https://github.com/trydofor/godbart/tree/master/demo/sql/

    2>&1 | tee /tmp/tmp.log                to save log
    | grep -E '^[0-9]{4}[^0-9][0-9]{2}'    to skip output
`

	//
	confFlag := &cli.StringFlag{
		Name:  "c",
		Usage: "the main (C)onfig `FILE`",
		Value: "godbart.toml",
	}

	destFlag := &cli.StringSliceFlag{
		Name:  "d",
		Usage: "the (D)estination `DB*` in config",
	}

	envsFlag := &cli.StringSliceFlag{
		Name:  "e",
		Usage: "the (E)nvironment, `K=v*`",
	}

	mlvlFlag := &cli.StringFlag{
		Name:  "l",
		Usage: "the message (L)evel, `debug?` :[debug|trace|error]",
		Value: "debug",
	}

	maskFlag := &cli.StringFlag{
		Name:  "m",
		Usage: "the (M)ask `regexp?` of the revision",
		Value: "[0-9]{10,}",
	}

	rqryFlag := &cli.StringFlag{
		Name:  "q",
		Usage: "the (Q)uery Prefix `string?` of revision",
		Value: "SELECT",
	}

	reviFlag := &cli.StringFlag{
		Name:  "r",
		Usage: "the (R)evision `string` to run to",
	}

	srceFlag := &cli.StringFlag{
		Name:  "s",
		Usage: "the (S)ource `DB` in config",
	}

	difkFlag := &cli.StringFlag{
		Name:  "t",
		Usage: "diff (T)ype,`type?` in,\n\tcol:columns\n\ttbl:table name\n\ttrg:trigger\n\t",
		Value: "tbl",
	}

	shwkFlag := &cli.StringFlag{
		Name:  "t",
		Usage: "show (T)emplet,`templet?` in config's sqltemplet",
		Value: "tbl,trg",
	}

	synkFlag := &cli.StringFlag{
		Name:  "t",
		Usage: "sync (T)ype `type?` in,\n\ttrg:trigger\n\ttbl:col+idx\n\trow:sync data\n\t",
		Value: "tbl",
	}

	sufxFlag := &cli.StringSliceFlag{
		Name:  "x",
		Usage: "the Suffi(X) `string?` of SQL files. eg \".sql\"",
	}

	riskFlag := &cli.BoolFlag{
		Name:  "agree",
		Usage: "dangerous SQL can lost data, you agree to take any risk on yourself!",
	}

	//
	app.Commands = []cli.Command{
		{
			Name:      "exec",
			Usage:     "execute SQLs on DBs",
			ArgsUsage: "some files or paths of SQLs",
			Flags: []cli.Flag{
				confFlag,
				sufxFlag,
				destFlag,
				mlvlFlag,
				riskFlag,
			},
			Action: exec,
		},
		{
			Name:      "revi",
			Usage:     "upgrade schema by revision",
			ArgsUsage: "some files or paths of SQLs",
			Description:`
			# save all tables(exclde $) create-table to prd_main_tbl.sql
			godbart show -s prd_main -t tbl 'tx_[^s]+' > prd_main_tbl.sql
			# save all tables(exclde $) create-trigger to prd_main_trg.sql
			godbart show -s prd_main -t trg 'tx_[^s]+' > prd_main_trg.sql
			`,
			Flags: []cli.Flag{
				confFlag,
				sufxFlag,
				destFlag,
				reviFlag,
				maskFlag,
				rqryFlag,
				mlvlFlag,
				riskFlag,
			},
			Action: revi,
		},
		{
			Name:      "diff",
			Usage:     "diff table, column, index, trigger",
			ArgsUsage: "tables to diff (regexp). empty means all",
			Description:`
			# save all tables(exclde $) create-table to prd_main_tbl.sql
			godbart show -s prd_main -t tbl 'tx_[^s]+' > prd_main_tbl.sql
			# save all tables(exclde $) create-trigger to prd_main_trg.sql
			godbart show -s prd_main -t trg 'tx_[^s]+' > prd_main_trg.sql
			`,
			Flags: []cli.Flag{
				confFlag,
				srceFlag,
				destFlag,
				difkFlag,
				mlvlFlag,
			},
			Action: diff,
		},
		{
			Name:      "sync",
			Usage:     "create table d.A like s.B or sync small data",
			ArgsUsage: "tables to sync (regexp). empty means all",
			Description:`
			# sync table&trigger from main to 2018
			godbart sync -s prd_main -d prd_2018 -t tbl,trg 'tx_[^s]+'
			# sync data from main to 2018
			godbart sync -s prd_main -d prd_2018 -t row 'tx_[^s]+'
			`,
			Flags: []cli.Flag{
				confFlag,
				srceFlag,
				destFlag,
				synkFlag,
				mlvlFlag,
				riskFlag,
			},
			Action: synk,
		},
		{
			Name:      "tree",
			Usage:     "deal data-tree between DBs",
			ArgsUsage: "some files or paths of SQLs",
			Description:`
			# save all tables(exclde $) create-table to prd_main_tbl.sql
			godbart tree -s prd_main -d prd_2018 demo/sql/tree/tree.sql
			`,
			Flags: []cli.Flag{
				confFlag,
				sufxFlag,
				destFlag,
				srceFlag,
				envsFlag,
				mlvlFlag,
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
				mlvlFlag,
			},
			Action: sqlx,
		},
		{
			Name:      "show",
			Usage:     "show ddl of table",
			ArgsUsage: "tables to show (regexp). empty means all",
			Description:`
			# save all tables(exclde $) create-table to prd_main_tbl.sql
			godbart show -s prd_main -t tbl 'tx_[^s]+' > prd_main_tbl.sql
			# save all tables(exclde $) create-trigger to prd_main_trg.sql
			godbart show -s prd_main -t trg 'tx_[^s]+' > prd_main_trg.sql
			`,
			Flags: []cli.Flag{
				confFlag,
				srceFlag,
				shwkFlag,
				mlvlFlag,
			},
			Action: show,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		art.LogFatal("exit by error=%v", err)
	}
}
