package http

import (
	"encoding/json"
	"fmt"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/spf13/viper"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type (
	httpParams struct {
		HttpPort       int `mapstructure:"http_port"`
		ReadTimeout    int `mapstructure:"read_timeout"`
		WriteTimeout   int `mapstructure:"write_timeout"`
		MaxHeaderBytes int `mapstructure:"max_header_bytes"`
	}
)

var (
	httpConf      map[string]*httpParams
	httpServerMap map[string]*Server
	httpOnce      sync.Once
)

func initHttpConfig() {
	if viper.IsSet("http") {
		initHttpServer()
	}
	initClient()
}

func initHttpServer() {
	if err := viper.UnmarshalKey("http", &httpConf); err != nil {
		panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
	}
	httpServerMap = map[string]*Server{}
	for k, conf := range httpConf {
		if conf.HttpPort == 0 {
			log.SysLogger().Errorw("http port not config", "key", k)
		}
		if conf.ReadTimeout == 0 {
			conf.ReadTimeout = 10
		}
		if conf.WriteTimeout == 0 {
			conf.WriteTimeout = 10
		}
		if conf.MaxHeaderBytes == 0 {
			conf.MaxHeaderBytes = 1048576
		}
		httpServerMap[k] = &Server{
			HttpServer: &http.Server{
				Addr:           ":" + strconv.Itoa(conf.HttpPort),
				ReadTimeout:    time.Duration(conf.ReadTimeout) * time.Second,
				WriteTimeout:   time.Duration(conf.WriteTimeout) * time.Second,
				MaxHeaderBytes: conf.MaxHeaderBytes,
			},
			Name: k,
		}
	}
	bytes, _ := json.MarshalIndent(httpConf, "", "  ")
	log.SysLogger().Debugf("http config: \n%v", string(bytes))
}
