package data

import (
	"context"
	"github.com/go-redis/redis/v8"
	myredis "github.com/jiangxw06/go-butler/internal/components/redis"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
)

type (
	ocpTestSuite struct {
		client    redis.UniversalClient
		ctx       context.Context
		lockCount int
	}
)

var (
	ocpSuite ocpTestSuite
)

func initTestObjectCacheProxy() {
	env.MergeConfig(`
	[redis]
    poolsize = 300
    [redis.clusters.default]
        appid = 21095
        pass = "fbf92587972f4cdf8685d27848409e19"
`)
	client := myredis.MustGetRedisClient("default")
	ctx := contexts.SetupContext(nil)
	ocpSuite = ocpTestSuite{
		client: client,
		ctx:    ctx,
	}
}
