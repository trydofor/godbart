package main

import (
	"errors"
	my "github.com/trydofor/godbart/internal"
	"github.com/urfave/cli"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

func checkConf(ctx *cli.Context) *my.Config {
	file := checkPath(ctx, "conf")

	for _, f := range file {
		log.Printf("[TRACE] got conf=%s\n", f.Path)
		conf, err := my.ParseToml(f.Text)
		if err != nil {
			log.Fatalf("[ERROR] can not parse TOML, config=%s\n", f.Path)
			os.Exit(-1)
		}
		return conf
	}
	return nil
}

func checkDest(ctx *cli.Context, cnf *my.Config) []my.DataSource {
	flag := ctx.StringSlice("dest")
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

func checkPath(ctx *cli.Context, key string) (files []my.FileEntity) {

	flag := ctx.String(key)
	if flag == "" {
		log.Fatalf("[ERROR] must give a value for key=%s\n", key)
		os.Exit(-3)
	}

	// the function that handles each file or dir

	var ff = func(p string, f os.FileInfo, e error) error {

		if e != nil {
			log.Fatalf("[ERROR] error=%v at a path=%q\n", e, p)
			return e
		}

		if !f.IsDir() {
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

	err := filepath.Walk(flag, ff)
	if err != nil {
		log.Fatalf("[ERROR] failed to read file=%s\n", key)
		os.Exit(-3)
	}

	if len(files) < 1 {
		log.Fatalf("[ERROR] must give a file=%s\n", key)
		os.Exit(-3)
	}

	return
}

func checkSour(ctx *cli.Context, cnf *my.Config) my.DataSource {
	flag := ctx.String("source")
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
	flag := ctx.StringSlice("envs")

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

func exec(ctx *cli.Context) (err error) {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	dest := checkDest(ctx, conf)
	file := checkPath(ctx, "path")
	test := ctx.Bool("test")
	return my.Exec(&conf.Preference, dest, file, test)
}

func revi(ctx *cli.Context) (err error) {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	dest := checkDest(ctx, conf)
	file := checkPath(ctx, "path")
	revi := ctx.String("revi")
	test := ctx.Bool("test")
	if revi == "" {
		err = errors.New("need revi")
	}
	return my.Revi(&conf.Preference, dest, file, revi, test)
}

func diff(ctx *cli.Context) error {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	source := checkSour(ctx, conf)
	dest := checkDest(ctx, conf)
	test := ctx.Bool("test")
	return my.Diff(&conf.Preference, dest, &source, &conf.DiffSchema, test)
}

func move(ctx *cli.Context) error {
	conf := checkConf(ctx)
	conf.StartupEnv = checkEnvs(ctx)
	source := checkSour(ctx, conf)
	dest := checkDest(ctx, conf)
	test := ctx.Bool("test")
	return my.Move(&conf.Preference, dest, &source, &conf.TreeMoving, test)
}

func main() {

	app := cli.NewApp()

	app.Author = "github.com/trydofor"
	app.Version = "0.9.1"
	app.Compiled = time.Now()

	app.Name = "godbart"
	app.Usage = app.Name + " command args"
	app.Description = "SQL-based CLI for RDBMS schema versioning & data migration"

	confFlag := &cli.StringFlag{
		Name:  "conf, c",
		Usage: "the main config",
		Value: "godbart.toml",
	}

	sourceFlag := &cli.StringFlag{
		Name:  "source, s",
		Usage: "the source datasources in config",
	}

	testFlag := &cli.BoolFlag{
		Name:  "test, t",
		Usage: "only test and report, not realy run",
	}

	destFlag := &cli.StringSliceFlag{
		Name:  "dest, d",
		Usage: "the destination datasources in config",
	}

	envsFlag := &cli.StringSliceFlag{
		Name:  "envs, e",
		Usage: "the environment `-e 'DATE_FROM=2018-11-23 12:34:56'`",
	}

	pathFlag := &cli.StringFlag{
		Name:  "path, p",
		Usage: "the sql path or file to exec",
	}

	reviFlag := &cli.StringFlag{
		Name:  "revi, r",
		Usage: "the revision to run to",
	}

	app.Commands = []cli.Command{
		{
			Name:  "exec",
			Usage: "execute sql on dbs",
			Flags: []cli.Flag{
				confFlag,
				pathFlag,
				destFlag,
				envsFlag,
				testFlag,
			},
			Action: exec,
		},
		{
			Name:  "revi",
			Usage: "upgrade schema by revision",
			Flags: []cli.Flag{
				confFlag,
				pathFlag,
				destFlag,
				reviFlag,
				envsFlag,
				testFlag,
			},
			Action: revi,
		},
		{
			Name:  "diff",
			Usage: "diff column,index,trigger",
			Flags: []cli.Flag{
				confFlag,
				sourceFlag,
				destFlag,
				envsFlag,
				testFlag,
			},
			Action: diff,
		},
		{
			Name:  "move",
			Usage: "move data tree to other db",
			Flags: []cli.Flag{
				confFlag,
				sourceFlag,
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
