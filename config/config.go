package config

import (
	"github.com/shibukawa/configdir"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"sync"
)

var gConfig *Config
var configOnce sync.Once

type Config struct {
	TwitterConfig `yaml:"twitter"`
}

type TwitterConfig struct {
	ConsumerKey    string   `yaml:"consumerKey"`
	ConsumerSecret string   `yaml:"consumerSecret"`
	Lists          []string `yaml:"lists,flow"`
	Hashtags       []string `yaml:"hashtags,flow"`
}

func ConfigPath() string {
	configDirs := configdir.New("SocialNetworksNews", "API")
	filePath := filepath.ToSlash(configDirs.QueryFolders(configdir.Global)[0].Path)
	return filePath
}

func GetConfig() (*Config, error) {
	var gFerr error
	configOnce.Do(func() {
		config := &Config{}
		filePath := ConfigPath()
		b, err := ioutil.ReadFile(filepath.Join(filePath, "config.yaml")) // just pass the file name
		if err != nil {
			gFerr = err
			return
		}

		err = yaml.Unmarshal(b, config)
		if err != nil {
			gFerr = err
			return
		}

		gConfig = config
	})
	if gFerr != nil {
		return nil, gFerr
	}

	return gConfig, nil
}
