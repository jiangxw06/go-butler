package data

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	myredis "github.com/jiangxw06/go-butler/internal/components/redis"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"sync"
	"testing"
	"time"
)

type (
	cloudLockTestSuite struct {
		client    redis.UniversalClient
		ctx       context.Context
		lockCount int
	}
)

var (
	cloudLockTestSuite1 cloudLockTestSuite
)

func initTestCloudLock() {
	env.MergeConfig(`
	[redis]
    poolsize = 300
    [redis.clusters.default]
        appid = 21095
        pass = "fbf92587972f4cdf8685d27848409e19"
`)
	client := myredis.MustGetRedisClient("default")
	ctx := contexts.SetupContext(nil)
	cloudLockTestSuite1 = cloudLockTestSuite{
		client: client,
		ctx:    ctx,
	}
}

func TestCloudLock_Delay(t *testing.T) {
	initTestCloudLock()
	lock := NewCloudLock(cloudLockTestSuite1.client, "testKey")
	success, err := lock.Lock(cloudLockTestSuite1.ctx)
	fmt.Println(success, err)
}

func TestCloudLock_Lock(t *testing.T) {
	initTestCloudLock()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go worker(&wg)
	}
	wg.Wait()
	contexts.Logger(cloudLockTestSuite1.ctx).Infow("done", "count", cloudLockTestSuite1.lockCount)
	env.CloseComponents()
}

func worker(wg *sync.WaitGroup) {
	defer wg.Done()

	cloudLock := NewCloudLock(cloudLockTestSuite1.client, "testclock1")
	cloudLock.SetTryLockInterval(time.Millisecond * 10)
	cloudLock.SetExpiration(time.Millisecond * 100)
	cloudLock.SetTryLockTimeout(time.Second * 100)

	contexts.Logger(cloudLockTestSuite1.ctx).Info("getting lock")

	success, err := cloudLock.Lock(cloudLockTestSuite1.ctx)
	contexts.Logger(cloudLockTestSuite1.ctx).Info("got lock")
	if err != nil {
		fmt.Printf("lock err: %v\n", err)
		return
	}
	if !success {
		fmt.Println("lock failed")
	}
	defer func() {
		cloudLock.Unlock(cloudLockTestSuite1.ctx)
		contexts.Logger(cloudLockTestSuite1.ctx).Info("unlock lock")
	}()

	unsafeJob()
}

func unsafeJob() {
	a := cloudLockTestSuite1.lockCount
	time.Sleep(time.Millisecond * 90)
	cloudLockTestSuite1.lockCount = a + 1
}
