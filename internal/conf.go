package internal

import (
	"github.com/pelletier/go-toml"
	"strings"
)

type Config struct {
	Preference Preference
	DataSource map[string]DataSource
	DiffSchema DiffSchema
	TreeMoving TreeMoving
	StartupEnv map[string]string
}

type Preference struct {
	DatabaseType string
	DelimiterRaw string
	DelimiterCmd string
	Transaction  bool
	IgnoreError  bool
	LineComment  string
	MultComment  []string
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

type DiffSchema struct {
	IgnoreTable []string
	IgnoreFiled []string
	IgnoreIndex []string
	ingoreTrigr []string
}

type TreeMoving struct {
	None []string
	Move []string
	Copy []string
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
	transaction := prefTree.Get("transaction").(bool)
	ignoreerror := prefTree.Get("ignoreerror").(bool)
	linecomment := prefTree.Get("linecomment").(string)
	multcomment := toArrString(prefTree.Get("multcomment"))
	connmaxopen := prefTree.Get("connmaxopen").(int64)
	connmaxidel := prefTree.Get("connmaxidel").(int64)

	//
	dsTree := conf.Get("datasource").(*toml.Tree)
	dataSource := toDataSource(dsTree.ToMap())

	//
	diffTree := conf.Get("diffschema").(*toml.Tree)
	ignoreTable := toArrString(diffTree.Get("ignore_table"))
	ignoreField := toArrString(diffTree.Get("ignore_filed"))
	ignoreIndex := toArrString(diffTree.Get("ignore_index"))
	ignoreTrigr := toArrString(diffTree.Get("ingore_trigr"))

	//
	treeTree := conf.Get("treemoving").(*toml.Tree)
	noneArr := toArrString(treeTree.Get("none"))
	moveArr := toArrString(treeTree.Get("move"))
	copyArr := toArrString(treeTree.Get("copy"))

	config = &Config{}
	config.Preference = Preference{
		databasetype,
		delimiterraw,
		delimitercmd,
		transaction,
		ignoreerror,
		linecomment,
		multcomment,
		int(connmaxopen),
		int(connmaxidel),
	}
	config.DataSource = dataSource
	config.DiffSchema = DiffSchema{
		ignoreTable,
		ignoreField,
		ignoreIndex,
		ignoreTrigr}
	config.TreeMoving = TreeMoving{
		noneArr,
		moveArr,
		copyArr}

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
