package etcd

import (
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/jiangxw06/go-butler/internal/env"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/spf13/viper"
	"sync"
	"time"
)

type (
	etcdConfig struct {
		Environment string
		Addrs       []string
	}
)

var (
	etcdClient *clientv3.Client
	etcdConf   etcdConfig
	etcdOnce   sync.Once
)

func initEtcdClient() {
	if !viper.IsSet("etcd") {
		log.SysLogger().Error("etcd not config in file")
		return
	}
	if err := viper.UnmarshalKey("etcd", &etcdConf); err != nil {
		panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
	}

	defer func() {
		//收到终止信号后unregister
		env.AddShutdownFunc(func() error {
			log.SysLogger().Info("closing etcdClient...")
			err := etcdClient.Close()
			if err != nil {
				log.SysLogger().Errorw("close etcdClient error", "error", err)
			} else {
				log.SysLogger().Info("close etcdClient success")
			}
			return nil
		})
	}()

	var err error
	for i := 0; i < 3; i++ {
		etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   etcdConf.Addrs,
			DialTimeout: time.Second,
		})
		if err == nil {
			log.SysLogger().Info("connect etcd success")
			return
		} else {
			log.SysLogger().Errorw("init etcd client error", "err", err)
		}
	}
	log.SysLogger().Errorw("init etcd client failed after retry")
}

func GetEtcdKey(suffix string) string {
	etcdOnce.Do(initEtcdClient)
	return fmt.Sprintf("/go/%v/%v", etcdConf.Environment, suffix)
}

func GetEtcdClient() *clientv3.Client {
	etcdOnce.Do(initEtcdClient)
	return etcdClient
}
