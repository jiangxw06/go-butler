package common

import (
	http2 "github.com/jiangxw06/go-butler/internal/components/http"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

//Init 启动common初始化
//可以用configPaths传入配置文件的path
//没有参数时优先从环境变量CONFIG_FILE_PATH读取path
//若都没有则使用默认路径./config
//配置文件名必须是app.toml
func Init() {
}

func ReadConfig(configStr string) {
	env.MergeConfig(configStr)
}

func ReadConfigFile(configFile string) {
	env.MergeConfigFile(configFile)
}

//StartPromServer 启动一个http服务器暴露本服务的prometheus指标
func StartPromServer() {
	promServer, err := http2.GetServer("prometheus")
	if err != nil {
		log.SysLogger().Error("StartPromServer failed", "err", "[http.prometheus] not config")
		return
	}

	mux := http.DefaultServeMux
	mux.Handle("/metrics", promhttp.Handler())

	promServer.HttpServer.Handler = mux

	go http2.StartHttpServer(promServer)
}

//Close 关闭应用，触发所有组件注册的关闭回调函数
func Close() {
	log.SysLogger().Infof("application closing")
	doClose()
}

//WaitClose 等待OS的关闭信号，然后关闭应用
func WaitClose() {
	log.SysLogger().Info("waiting signal to close...")
	s := env.WaitClose()
	log.SysLogger().Infof("receive signal: %v, application closing", s.String())
	doClose()

}

func doClose() {
	errs := env.CloseComponents()
	for _, err := range errs {
		log.SysLogger().Errorw("err during closing components", "err", err)
	}
	log.SysLogger().Info("application closed")
	log.CloseSysLogger()
}
