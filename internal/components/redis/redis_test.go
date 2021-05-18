package redis

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"math"
	"os"
	"testing"
	"time"
)

var (
	client redis.UniversalClient
	ctx    context.Context
)

func TestMain(m *testing.M) {
	env.MergeConfig(`
[redis]
    poolsize = 300
    [redis.clusters.default]
        appid = 21095
        pass = "fbf92587972f4cdf8685d27848409e19"
    [redis.clusters.second]
        appid = 20716
        pass = "03efa3d51a594480a484ec36653a2178"
`)

	flag.Parse()
	client = MustGetRedisClient("default")
	ctx = contexts.SetupContext(nil)
	exitCode := m.Run()

	// 退出
	os.Exit(exitCode)
}

func TestRedis(t *testing.T) {
	res, err := client.Set(ctx, "test", []byte{}, time.Hour).Result()
	//res, err := cli.Load("test").Result()
	if err == redis.Nil {
		log.SysLogger().Info("key not exist")
	} else {
		log.SysLogger().Infow("test", "res", res, "err", err)
	}
	log.CloseSysLogger()
}

type (
	TmpStruct struct {
		A int
		B string
	}
)

func TestJson(t *testing.T) {
	tmp := TmpStruct{
		A: 10,
		B: "abc",
	}
	bytes, err := json.Marshal(&tmp)
	fmt.Printf("%v, %v", string(bytes), err)
}

func TestInf(t *testing.T) {
	tmp := math.Inf(-1)
	log.SysLogger().Infof("%v", tmp)
	log.CloseSysLogger()
}

func TestRange(t *testing.T) {
	res, err := client.ZRangeWithScores(ctx, "ztest", 0, -1).Result()
	for i, ele := range res {
		log.SysLogger().Infof("res %v: %T %v", i, ele.Member, ele.Member.(string))
	}
	log.SysLogger().Infof("%v %v", res, err)
	log.CloseSysLogger()
}
