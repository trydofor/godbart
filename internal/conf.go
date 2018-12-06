package internal

import (
	"github.com/pelletier/go-toml"
	"strings"
)

type Config struct {
	Preference Preference
	DataSource map[string]DataSource
	StartupEnv map[string]string
}

type Preference struct {
	DatabaseType string
	DelimiterRaw string
	DelimiterCmd string
	LineComment  string
	MultComment  []string
	FmtDateTime  string
	ConnMaxOpen  int
	ConnMaxIdel  int
}

type FileEntity struct {
	Path string
	Text string
}

type DataSource struct {
	Code string
	Conn string
}

//

func ParseToml(text string) (config *Config, err error) {

	conf, err := toml.Load(text)
	if err != nil {
		return
	}

	prefTree := conf.Get("preference").(*toml.Tree)
	databasetype := prefTree.Get("databasetype").(string)
	delimiterraw := prefTree.Get("delimiterraw").(string)
	delimitercmd := prefTree.Get("delimitercmd").(string)
	linecomment := prefTree.Get("linecomment").(string)
	multcomment := toArrString(prefTree.Get("multcomment"))
	fmtdatetime := prefTree.Get("fmtdatetime").(string)
	connmaxopen := prefTree.Get("connmaxopen").(int64)
	connmaxidel := prefTree.Get("connmaxidel").(int64)

	//
	dsTree := conf.Get("datasource").(*toml.Tree)
	dataSource := toDataSource(dsTree.ToMap())

	config = &Config{}
	config.Preference = Preference{
		databasetype,
		delimiterraw,
		delimitercmd,
		linecomment,
		multcomment,
		fmtdatetime,
		int(connmaxopen),
		int(connmaxidel),
	}
	config.DataSource = dataSource

	return
}

func toDataSource(m map[string]interface{}) map[string]DataSource {
	rst := make(map[string]DataSource)
	for k, v := range m {
		switch v.(type) {
		case string:
			rst[k] = DataSource{k, v.(string)}
		}
	}
	return rst
}

func toArrString(a interface{}) []string {
	arr := a.([]interface{})
	rst := make([]string, len(arr))
	for i, j := 0, 0; i < len(arr); i++ {
		switch arr[i].(type) {
		case string:
			s := strings.TrimSpace(arr[i].(string))
			if len(s) > 0 {
				rst[j] = s
				j++
			}
		}
	}
	return rst
}
