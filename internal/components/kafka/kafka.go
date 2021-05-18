package kafka

import (
	"encoding/json"
	"fmt"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/spf13/viper"
	"sync"
)

type (
	kafkaConfig struct {
		Clusters map[string][]string
		Topics   map[string]*struct {
			Topic   string
			Cluster string
			Group   string
		}
	}
)

var (
	kafkaConf kafkaConfig
	kafkaOnce sync.Once
)

func loadKafkaConfig() {
	if viper.IsSet("kafka") {
		if err := viper.UnmarshalKey("kafka", &kafkaConf); err != nil {
			panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
		}
		for _, v := range kafkaConf.Topics {
			if v.Cluster == "" {
				v.Cluster = "default"
			}
			if !checkCluster(v.Cluster) {
				log.SysLogger().Warnf("kafka %v topics cluster %v not find in conf clusters", v.Topic, v.Cluster)
			}
		}
		bytes, _ := json.MarshalIndent(kafkaConf, "", "  ")
		log.SysLogger().Debugf("kafka config: \n%v", string(bytes))

		initProducers()
	}
}

func checkCluster(nowCluster string) bool {
	for cluster := range kafkaConf.Clusters {
		if cluster == nowCluster {
			return true
		}
	}
	return false
}
