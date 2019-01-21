package art

import (
	"errors"
	"github.com/pelletier/go-toml"
	"strings"
)

type Config struct {
	Preference Preference
	SqlTemplet map[string]string
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
	ControlPort  int
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

	preference, err := parsePreference(conf)
	if err != nil {
		return
	}
	sqltemplet, err := parseSqlTemplet(conf)
	if err != nil {
		return
	}
	datasource, err := parseDataSource(conf)
	if err != nil {
		return
	}

	config = &Config{
		preference,
		sqltemplet,
		datasource,
		make(map[string]string),
	}

	return
}

func parseSqlTemplet(conf *toml.Tree) (rst map[string]string, err error) {
	if tree, ok := conf.Get("sqltemplet").(*toml.Tree); ok {
		rst = make(map[string]string)
		for k, v := range tree.ToMap() {
			switch v.(type) {
			case string:
				rst[k] = v.(string)
			default:
				err = errors.New("unsupported value, sqltemplet." + k);
				return
			}
		}
	} else {
		err = errorAndLog("failed to parse sqltemplet")
	}
	return
}

func parseDataSource(conf *toml.Tree) (rst map[string]DataSource, err error) {
	if tree, ok := conf.Get("datasource").(*toml.Tree); ok {
		rst = make(map[string]DataSource)
		for k, v := range tree.ToMap() {
			switch v.(type) {
			case string:
				rst[k] = DataSource{k, v.(string)}
			default:
				err = errors.New("unsupported value, sqltemplet." + k);
				return
			}
		}
	} else {
		err = errorAndLog("failed to parse datasource")
	}
	return
}

func parsePreference(conf *toml.Tree) (rst Preference, err error) {
	if tree, ok := conf.Get("preference").(*toml.Tree); ok {
		rst = Preference{
			toString(tree, "databasetype"),
			toString(tree, "delimiterraw"),
			toString(tree, "delimitercmd"),
			toString(tree, "linecomment"),
			toArrString(tree, "multcomment"),
			toString(tree, "fmtdatetime"),
			toInt(tree, "controlport"),
			toInt(tree, "connmaxopen"),
			toInt(tree, "connmaxidel"),
		}
	} else {
		err = errorAndLog("failed to parse preference")
	}
	return
}

func toInt(tree *toml.Tree, key string) (rst int) {
	if num, ok := tree.Get(key).(int64); ok {
		rst = int(num)
	} else {
		LogError("failed to get int, key=%s", key)
	}
	return
}

func toString(tree *toml.Tree, key string) (rst string) {
	if str, ok := tree.Get(key).(string); ok {
		rst = str
	} else {
		LogError("failed to get string, key=%s", key)
	}
	return
}

func toArrString(tree *toml.Tree, key string) (rst []string) {
	if arr, ok := tree.Get(key).([]interface{}); ok {
		rst = make([]string, len(arr))
		for i, j := 0, 0; i < len(arr); i++ {
			switch arr[i].(type) {
			case string:
				s := strings.TrimSpace(arr[i].(string))
				if len(s) > 0 {
					rst[j] = s
					j++
				}
			default:
				LogError("get unsupported type while parsing key=%s", key)
			}
		}
	} else {
		LogError("failed to get array, key=%s", key)
	}
	return
}
