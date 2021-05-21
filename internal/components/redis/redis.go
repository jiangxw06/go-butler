package redis

import (
	"encoding/json"
	"fmt"
	"github.com/jiangxw06/go-butler/internal/env"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type (
	//addressList  []redisAddress
	//redisAddress struct {
	//	IP          string `json:"ip"`
	//	Master      int32  `json:"master"`
	//	Persistence int32  `json:"persistence"`
	//	Port        int32  `json:"port"`
	//}
	redisClusterConfig struct {
		//RedisUID string `mapstructure:"appid"` // Redis实例ID
		Password string `mapstructure:"pass"`
		Addrs    []string
	}
	redisConfig struct {
		// Default is 8.
		MaxRedirects int `mapstructure:"maxredirects"`
		// Enables read-only commands on slave nodes.
		ReadOnly       bool `mapstructure:"readonly"`
		RouteByLatency bool `mapstructure:"routerbylatency"`
		RouteRandomly  bool `mapstructure:"routerrandomly"`
		// Default is to not retry failed commands.
		MaxRetries int `mapstructure:"maxretries"`
		// Dial timeout for establishing new connections. unit: milliseconds
		// Default is 5 seconds.
		// No timeout should set -1.
		DialTimeout int `mapstructure:"dialtimeout"`
		// Timeout for socket reads. If reached, commands will fail with a timeout instead of blocking. unit: milliseconds
		// Default is 3 seconds.
		// No timeout should set -1.
		ReadTimeout int `mapstructure:"readtimeout"`
		// Timeout for socket writes. If reached, commands will fail with a timeout instead of blocking. unit: milliseconds
		// Default is ReadTimeout.
		// No timeout should set -1.
		WriteTimeout int `mapstructure:"writetimeout"`
		// Amount of time after which client closes idle connections. unit: seconds
		// Should be less than server's timeout.
		// Default is 5 minutes.
		IdleTimeout int `mapstructure:"idletimeout"`
		// PoolSize applies per cluster node and not for the whole cluster.
		PoolSize           int           `mapstructure:"poolsize"`
		PoolTimeout        time.Duration `mapstructure:"pooltimeout"`
		IdleCheckFrequency time.Duration `mapstructure:"idlecheckfrequency"`

		Clusters map[string]redisClusterConfig
	}
)

var (
	redisConf = redisConfig{
		MaxRedirects:       5,
		ReadOnly:           false,
		RouteByLatency:     false,
		RouteRandomly:      false,
		MaxRetries:         5,
		DialTimeout:        500,
		ReadTimeout:        500,
		WriteTimeout:       500,
		IdleTimeout:        240,
		PoolSize:           1000,
		PoolTimeout:        500,
		IdleCheckFrequency: 60,
	}
	redisOnce sync.Once
)

func loadRedisConfig() {
	if viper.IsSet("redis") {
		if err := viper.UnmarshalKey("redis", &redisConf); err != nil {
			panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
		}
		bytes, _ := json.MarshalIndent(redisConf, "", "  ")
		log.SysLogger().Debugf("redis config: \n%v", string(bytes))
	}
}

func MustGetRedisClient(key string) redis.UniversalClient {
	cli, err := GetRedisClient(key)
	if err != nil {
		panic(err)
	}
	return cli
}

func GetRedisClient(key string) (redis.UniversalClient, error) {
	redisOnce.Do(loadRedisConfig)
	clusterConfig, ok := redisConf.Clusters[key]
	if !ok {
		err := errors.Errorf("redis client %v no config", key)
		//log.SysLogger().Error(err)
		return nil, err
	}
	clusterOptions := &redis.ClusterOptions{
		MaxRedirects:       redisConf.MaxRedirects,
		ReadOnly:           redisConf.ReadOnly,
		RouteByLatency:     redisConf.RouteByLatency,
		RouteRandomly:      redisConf.RouteByLatency,
		Password:           clusterConfig.Password,
		Addrs:              clusterConfig.Addrs,
		MaxRetries:         redisConf.MaxRetries,
		DialTimeout:        time.Duration(redisConf.DialTimeout) * time.Millisecond,
		ReadTimeout:        time.Duration(redisConf.ReadTimeout) * time.Millisecond,
		WriteTimeout:       time.Duration(redisConf.WriteTimeout) * time.Millisecond,
		IdleTimeout:        time.Duration(redisConf.IdleTimeout) * time.Second,
		PoolSize:           redisConf.PoolSize,
		PoolTimeout:        redisConf.PoolTimeout * time.Millisecond,
		IdleCheckFrequency: redisConf.IdleCheckFrequency * time.Second,
	}
	//var addrs []string
	//if len(clusterConfig.RedisUID) > 0 {
	//	addressList, err := getAddressList(clusterConfig.RedisUID)
	//	if err != nil {
	//		err := errors.Errorf("get redis-cluster address from panther error: %v", err)
	//		log.SysLogger().Error(err)
	//		return nil, err
	//	}
	//	if addressList != nil && len(*addressList) > 0 {
	//		for _, address := range *addressList {
	//			if len(address.IP) > 0 && address.Port > 0 && address.Master == 1 {
	//				addrs = append(addrs, fmt.Sprintf("%s:%v", address.IP, address.Port))
	//			}
	//		}
	//	}
	//}
	//clusterOptions.Addrs = addrs
	clusterClient := redis.NewClusterClient(clusterOptions)

	env.AddShutdownFunc(func() error {
		log.SysLogger().Infow("redis closing...", "key", key)
		err := clusterClient.Close()
		if err != nil {
			log.SysLogger().Errorw("close redis err", "err", err, "key", key)
		} else {
			log.SysLogger().Infow("close redis success", "key", key)
		}
		return nil
	})
	return clusterClient, nil
}

//func getAddressList(uid string) (*addressList, error) {
//	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
//	defer cancel()
//	resp, err := ctxhttp.Get(ctx, http.DefaultClient, defaultGetAddressListApi+uid)
//	if err != nil {
//		return nil, err
//	}
//	if resp != nil {
//		defer resp.Body.Close()
//	}
//	if resp.StatusCode != http.StatusOK {
//		return nil, errors.New(resp.Status)
//	}
//	respBody, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		return nil, err
//	}
//	var respParams = &addressList{}
//	var j = jsoniter.ConfigCompatibleWithStandardLibrary
//	err = j.Unmarshal(respBody, respParams)
//	if err != nil {
//		return nil, err
//	}
//	return respParams, nil
//}
