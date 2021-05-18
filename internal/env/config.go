package env

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"testing"
)

const (
	configFileFlag    = "config_file"
	configFileEnvVar  = "COMMON_CONFIG_FILE"
	donfigFileDefault = "./config/app.toml"
)

func init() {
	var configFile string

	//用程序参数读取configFile
	testing.Init() //为了解决Mac/Linux下go test在common.Init()之后启动导致common.Init()时找不到-test.V flag配置的问题
	flag.StringVar(&configFile, configFileFlag, "", "input config file path & name, eg: ./config/app.toml")
	flag.Parse()
	if configFile != "" {
		MergeConfigFile(configFile)
		return
	}

	//用环境变量读取configFile
	configFile = os.Getenv(configFileEnvVar)
	if configFile != "" {
		MergeConfigFile(configFile)
		return
	}

	//用默认路径读取configFile
	_, err := os.Stat(donfigFileDefault)
	if err != nil {
		if os.IsExist(err) {
			//文件路径存在
			MergeConfigFile(donfigFileDefault)
		}
		return
	}
	//文件路径存在
	MergeConfigFile(donfigFileDefault)
}

func MergeConfigFile(filePath string) {
	//dir, file := filepath.Split(filePath)
	//fileExt := filepath.Ext(file)
	//filePrefix := file[:len(file) - len(fileExt)]
	//viper.AddConfigPath(dir)
	viper.SetConfigFile(filePath)
	err := viper.MergeInConfig()
	if err != nil {
		fmt.Printf("mergeConfig err: %v\n", err)
	}
	//initEnv()
}

func MergeConfig(configStr string) {
	viper.SetConfigType("toml")
	err := viper.MergeConfig(bytes.NewBuffer([]byte(configStr)))
	if err != nil {
		fmt.Printf("mergeConfig err: %v\n", err)
	}
}
