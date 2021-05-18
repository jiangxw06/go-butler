package rocketmq

import (
	"encoding/json"
	"fmt"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/spf13/viper"
	"sync"
)

type (
	rocketMQConfig struct {
		Topics map[string]*topic
	}

	topic struct {
		Topic   string
		Cluster string
		Group   string
	}
)

var (
	rocketMQConf rocketMQConfig

	//在这里增加name server
	rocketMQCluster2NS = map[string]string{}

	rocketOnce sync.Once
)

func loadRocketMQConfig() {
	if viper.IsSet("rocketmq") {
		if err := viper.UnmarshalKey("rocketmq", &rocketMQConf); err != nil {
			panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
		}
		bytes, _ := json.MarshalIndent(rocketMQConf, "", "  ")
		log.SysLogger().Debugf("rocketMQConf config: \n%v", string(bytes))

		initRocketMQProducers()
	}
}
