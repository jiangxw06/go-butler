package data

import (
	"context"
	"time"

	"github.com/jiangxw06/go-butler/internal/log"

	"github.com/go-redis/redis/v8"
	"github.com/teris-io/shortid"
)

type (
	//CloudLock Redis实现的分布式锁，非线程安全
	CloudLock struct {
		RedisKey        string
		lockID          string
		RedisClient     redis.UniversalClient
		expiration      time.Duration
		tryLockInterval time.Duration
		tryLockTimeout  time.Duration
	}
)

//NewCloudLock 创建分布式锁
func NewCloudLock(redisClient redis.UniversalClient, resourceKey string) *CloudLock {
	cache := &CloudLock{}

	cache.RedisKey = "lock:" + resourceKey
	cache.RedisClient = redisClient
	cache.lockID, _ = shortid.Generate()

	cache.expiration = time.Second * 1
	cache.tryLockInterval = time.Millisecond * 10
	cache.tryLockTimeout = time.Second * 10

	return cache
}

//SetExpiration 设置锁的过期时间
func (l *CloudLock) SetExpiration(expiration time.Duration) *CloudLock {
	if expiration > 0 {
		l.expiration = expiration
	}
	return l
}

//SetTryLockInterval 设置尝试获取锁的间隔时间
func (l *CloudLock) SetTryLockInterval(interval time.Duration) *CloudLock {
	if interval > 0 {
		l.tryLockInterval = interval
	}
	return l
}

//SetTryLockInterval 设置尝试获取锁的间隔时间
func (l *CloudLock) SetTryLockTimeout(timeout time.Duration) *CloudLock {
	if timeout > 0 {
		l.tryLockTimeout = timeout
	}
	return l
}

//TryLock 尝试获得锁，若失败或error则立即返回
func (l *CloudLock) TryLock(ctx context.Context) (bool, error) {
	//setnx 成功返回1， 失败返回0， err返回错误信息
	success, err := l.RedisClient.SetNX(ctx, l.RedisKey, l.lockID, l.expiration).Result()
	if err != nil {
		log.SysLogger().Errorw("CloudLock tryLock err", "err", err, "key", l.RedisKey)
	}
	return success, err
}

//Lock 阻塞等待分布式锁，直到拿到锁或者error
func (l *CloudLock) Lock(ctx context.Context) (success bool, err error) {
	success, err = l.TryLock(ctx)
	if success || err != nil {
		return success, err
	}

	timer := time.NewTimer(l.tryLockTimeout)
	ticker := time.NewTicker(l.tryLockInterval)
	for {
		select {
		case <-timer.C:
			return false, nil
		case <-ticker.C:
			success, err = l.TryLock(ctx)
			if success || err != nil {
				return success, err
			}
		}
	}
}

//Unlock 解除自己加的锁
func (l *CloudLock) Unlock(ctx context.Context) {
	script := `
	if ( redis.call("exists",KEYS[1]) == 1 )
	then
		if ( redis.call("get", KEYS[1]) == ARGV[1] )
		then
			redis.call("del", KEYS[1])
			return 1
		else
			return 2
		end
	else
		return 3
	end
`
	_, err := l.RedisClient.Eval(ctx, script, []string{l.RedisKey}, l.lockID).Int()
	//common.SysLogger().Infow("CloudLock unlock return", "res", res)
	if err != nil {
		log.SysLogger().Errorw("CloudLock unlock err", "err", err, "key", l.RedisKey)
	}
}
