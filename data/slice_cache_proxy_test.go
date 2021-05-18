package data

import (
	"context"
	"github.com/go-redis/redis/v8"
	myredis "github.com/jiangxw06/go-butler/internal/components/redis"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/magiconair/properties/assert"
	"math"
	"testing"
	"time"
)

type (
	scpTestSuite struct {
		client redis.UniversalClient
		ctx    context.Context
		scp    *SliceCacheProxy
	}
	TestModel struct {
		P1 string `json:"pn1"`
		P2 int
		P3 []byte
	}
)

var (
	scpSuite scpTestSuite
)

func initTestSliceCacheProxy() {
	env.MergeConfig(`
	[redis]
    poolsize = 300
    [redis.clusters.default]
        appid = 21095
        pass = "fbf92587972f4cdf8685d27848409e19"
`)
	client := myredis.MustGetRedisClient("default")
	ctx := contexts.SetupContext(nil)
	scp := NewSliceCacheProxy(client, "test:scp:%v").SetRangeOrder(HighToLowOrder).SetCacheCapacity(1).SetExpiration(time.Minute * 5).
		SetDataSource(func(ctx context.Context, id interface{}, info RangeInfo) (members []*redis.Z, err error) {
			if id.(string) == "struct" {
				members = append(members, &redis.Z{
					Score: 3,
					Member: TestModel{
						P1: "name3",
						P2: 3,
						P3: []byte{3, 6, 9},
					},
				})
				members = append(members, &redis.Z{
					Score: 2,
					Member: TestModel{
						P1: "name2",
						P2: 2,
						P3: []byte{1, 2, 3},
					},
				})

			}
			if id.(string) == "bytes" {
				members = append(members, &redis.Z{
					Score:  3,
					Member: []byte{3, 6, 9},
				})
				members = append(members, &redis.Z{
					Score:  2,
					Member: []byte{1, 2, 3},
				})
			}
			if id.(string) == "int" {
				members = append(members, &redis.Z{
					Score:  3,
					Member: int(3),
				})
				members = append(members, &redis.Z{
					Score:  2,
					Member: int(2),
				})
			}
			if id.(string) == "float" {
				members = append(members, &redis.Z{
					Score:  3,
					Member: 3.1,
				})
				members = append(members, &redis.Z{
					Score:  2,
					Member: 2.1,
				})
			}
			if id.(string) == "string" {
				members = append(members, &redis.Z{
					Score:  3,
					Member: "3",
				})
				members = append(members, &redis.Z{
					Score:  2,
					Member: "2",
				})
			}
			return
		})
	scpSuite = scpTestSuite{
		client: client,
		ctx:    ctx,
		scp:    scp,
	}
}

func testslicecacheproxyReloadredis() {
	scpSuite.scp.reloadRedis(context.Background(), "struct")
	scpSuite.scp.reloadRedis(context.Background(), "bytes")
	scpSuite.scp.reloadRedis(context.Background(), "int")
	scpSuite.scp.reloadRedis(context.Background(), "float")
	scpSuite.scp.reloadRedis(context.Background(), "string")
}

func TestSliceCacheProxy_ReloadRedisIfNotExists(t *testing.T) {
	initTestSliceCacheProxy()
	scpSuite.scp.reloadRedisIfNotExists(context.Background(), "struct")
}

func TestSliceCacheProxy_Get(t *testing.T) {
	initTestSliceCacheProxy()
	scpSuite.scp.SetCacheCapacity(100)
	testslicecacheproxyReloadredis()
	testSliceCacheProxy_Get(t)
}

func TestSliceCacheProxy_Get2(t *testing.T) {
	initTestSliceCacheProxy()
	scpSuite.scp.SetCacheCapacity(1)
	testslicecacheproxyReloadredis()
	testSliceCacheProxy_Get(t)
}

func testSliceCacheProxy_Get(t *testing.T) {
	var res []TestModel
	_ = scpSuite.scp.Get(scpSuite.ctx, "struct", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &res)
	assert.Equal(t, res[0].P3, []byte{3, 6, 9})
	assert.Equal(t, res[1].P3, []byte{1, 2, 3})

	var byteList [][]byte
	_ = scpSuite.scp.Get(scpSuite.ctx, "bytes", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &byteList)
	assert.Equal(t, byteList[0], []byte{3, 6, 9})
	assert.Equal(t, byteList[1], []byte{1, 2, 3})

	var ints []int
	_ = scpSuite.scp.Get(scpSuite.ctx, "int", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &ints)
	assert.Equal(t, ints[0], 3)
	assert.Equal(t, ints[1], 2)

	var floats []float64
	_ = scpSuite.scp.Get(scpSuite.ctx, "float", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &floats)
	assert.Equal(t, floats[0], 3.1)
	assert.Equal(t, floats[1], 2.1)

	var strs []string
	_ = scpSuite.scp.Get(scpSuite.ctx, "string", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &strs)
	assert.Equal(t, strs[0], "3")
	assert.Equal(t, strs[1], "2")
}

func TestSliceCacheProxy_GetFromSource(t *testing.T) {
	initTestSliceCacheProxy()
	var res []TestModel
	_ = scpSuite.scp.GetFromSource(scpSuite.ctx, "struct", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &res)
	assert.Equal(t, res[0].P3, []byte{3, 6, 9})
	assert.Equal(t, res[1].P3, []byte{1, 2, 3})

	var byteList [][]byte
	_ = scpSuite.scp.GetFromSource(scpSuite.ctx, "bytes", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &byteList)
	assert.Equal(t, byteList[0], []byte{3, 6, 9})
	assert.Equal(t, byteList[1], []byte{1, 2, 3})

	var ints []int
	_ = scpSuite.scp.GetFromSource(scpSuite.ctx, "int", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &ints)
	assert.Equal(t, ints[0], 3)
	assert.Equal(t, ints[1], 2)

	var floats []float64
	_ = scpSuite.scp.GetFromSource(scpSuite.ctx, "float", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &floats)
	assert.Equal(t, floats[0], 3.1)
	assert.Equal(t, floats[1], 2.1)

	var strs []string
	_ = scpSuite.scp.GetFromSource(scpSuite.ctx, "string", RangeInfo{
		StartScore:        math.Inf(1),
		IncludeStartScore: false,
		Offset:            0,
		Limit:             100,
	}, &strs)
	assert.Equal(t, strs[0], "3")
	assert.Equal(t, strs[1], "2")
}
