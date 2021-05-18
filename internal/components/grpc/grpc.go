package grpc

import (
	"encoding/json"
	"fmt"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/spf13/viper"
	"sync"
)

type (
	svcConfig struct {
		Dependencies []string
		Register     map[string]struct {
			Port int
		}
	}
)

var (
	serviceConf svcConfig
	svcOnce     sync.Once
)

func loadSvcConfig() {
	if viper.IsSet("service") {
		if err := viper.UnmarshalKey("service", &serviceConf); err != nil {
			panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
		}
		bytes, _ := json.MarshalIndent(serviceConf, "", "  ")
		log.SysLogger().Debugf("service config: \n%v", string(bytes))
	}
}
