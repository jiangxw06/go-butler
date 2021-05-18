package data

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jiangxw06/go-butler"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"time"
)

const (
	NotExistsString = ""
)

type (
	ObjectCacheProxy struct {
		keyTemplate string
		expireTime  time.Duration //设置缓存过期时间，为0则永不过期
		redisClient redis.UniversalClient
		idMapping   func(daoMutation DaoMutation) (id interface{})
		dataSource  func(ctx context.Context, id interface{}) (object interface{}, err error)

		//实现DaoEventListener，异步删除陈旧的缓存key
		daoEventChan        chan DaoMutation
		daoEventChanClosed  bool
		cleanerClosed       chan struct{}
		asyncDelKeyInterval time.Duration

		//todo:实现防缓存击穿
		protectDB        bool
		lockExpiration   time.Duration //锁过期时间
		tryCacheInterval time.Duration //未拿到锁，定期访问缓存资源
		maxWaitTime      time.Duration //超过最大等待时间后直接访问db
	}
)

func (p *ObjectCacheProxy) Listen(event DaoMutation) {
	cacheCounters.WithLabelValues(p.keyTemplate, "db_mutation").Inc()
	if p.daoEventChanClosed {
		common.SysLogger().Warnw("receive daoMutation after closing objectCacheProxy chan", "keyTemplate", p.keyTemplate, "event", event)
	} else {
		p.daoEventChan <- event
	}
}

var (
	_ DaoEventListener = new(ObjectCacheProxy)
)

//NewStringGroup 生成一个对象缓存组实例
func NewObjectCacheProxy(redisClient redis.UniversalClient, keyTemplate string) *ObjectCacheProxy {
	cache := &ObjectCacheProxy{}
	cache.expireTime = time.Minute * 10
	cache.keyTemplate = keyTemplate
	cache.redisClient = redisClient

	cache.daoEventChan = make(chan DaoMutation, 1000)
	cache.asyncDelKeyInterval = time.Millisecond * 10
	cache.cleanerClosed = make(chan struct{})

	env.AddShutdownFunc(func() error {
		common.SysLogger().Infow("closing cacheProxy", "keyTemplate", cache.keyTemplate)
		//关闭daoEvent通道
		cache.daoEventChanClosed = true
		close(cache.daoEventChan)
		//等待cleaner结束
		<-cache.cleanerClosed
		common.SysLogger().Infow("cacheProxy closed", "keyTemplate", cache.keyTemplate)
		return nil
	})

	go cache.startStaleKeyCleaner()
	return cache
}

func (p *ObjectCacheProxy) SetExpiration(expiration time.Duration) *ObjectCacheProxy {
	p.expireTime = expiration
	return p
}

func (p *ObjectCacheProxy) SetStaleKeyCleaner(publisher DaoEventPublisher, mapping func(daoMutation DaoMutation) (id interface{})) *ObjectCacheProxy {
	publisher.AddListener(p)
	p.idMapping = mapping
	return p
}

//SetDataSource 设置从数据源查询数据的函数
func (p *ObjectCacheProxy) SetDataSource(f func(ctx context.Context, id interface{}) (object interface{}, err error)) *ObjectCacheProxy {
	p.dataSource = f
	return p
}

func (p *ObjectCacheProxy) delKeys(ids []interface{}) {
	pipe := p.redisClient.Pipeline()
	for _, id := range ids {
		key := p.assembleKey(id)
		pipe.Del(context.Background(), key)
	}
	cmders, err := pipe.Exec(context.Background())
	cacheCounters.WithLabelValues(p.keyTemplate, "redis_del").Inc()
	if err != nil {
		log.SysLogger().Errorw("ObjectCacheProxy delete key error", "keyTemplate", p.keyTemplate, "ids", ids, "err", err)
		return
	}
	var delCounts int64
	for _, cmder := range cmders {
		cmd := cmder.(*redis.IntCmd)
		delCount, err := cmd.Result()
		if err != nil {
			//按我的理解应该不会进来
			log.SysLogger().Errorw("ObjectCacheProxy err which shouldn't enter", "keyTemplate", p.keyTemplate, "ids", ids, "err", err)
		}
		delCounts += delCount
	}
	log.SysLogger().Debugw("ObjectCacheProxy delKeys", "keyTemplate", p.keyTemplate, "len(ids)", len(ids), "delCounts", delCounts)
}

func (p *ObjectCacheProxy) startStaleKeyCleaner() {
	ticker := time.NewTicker(p.asyncDelKeyInterval)
	var ids []interface{}
	defer func() {
		ticker.Stop()
		if len(ids) > 0 {
			p.delKeys(ids)
		}
		p.cleanerClosed <- struct{}{}
		common.SysLogger().Debugw("staleKeyCleaner stopped", "keyTemplate", p.keyTemplate)
	}()

	for {
		select {
		case <-ticker.C:
			if len(ids) > 0 {
				go p.delKeys(ids)
				ids = nil
			}
		case daoEvent, ok := <-p.daoEventChan:
			if ok {
				id := p.idMapping(daoEvent)
				ids = append(ids, id)
				if len(ids) >= 100 {
					go p.delKeys(ids)
					ids = nil
				}
			} else {
				//通道已关闭
				return
			}
		}
	}
}

func (p *ObjectCacheProxy) assembleKey(id interface{}) (key string) {
	key = fmt.Sprintf(p.keyTemplate, id)
	return
}

func (p *ObjectCacheProxy) Get(ctx context.Context, id interface{}, target interface{}) (found bool, err error) {
	cacheCounters.WithLabelValues(p.keyTemplate, "get").Inc()
	redisMiss := false
	begin := time.Now()
	defer func() {
		duration := time.Since(begin).Seconds()
		if redisMiss {
			cacheDurations.WithLabelValues(p.keyTemplate, "redis_miss").Observe(duration)
		} else {
			cacheDurations.WithLabelValues(p.keyTemplate, "redis_hit").Observe(duration)
		}
	}()

	redisResult, err := p.redisClient.Get(ctx, p.assembleKey(id)).Result()
	if err == nil {
		if redisResult == NotExistsString {
			//统计
			cacheCounters.WithLabelValues(p.keyTemplate, "redis_empty").Inc()
			return false, nil
		}
		err = json.Unmarshal([]byte(redisResult), target)
		//统计
		common.Logger(ctx).Debugw("ObjectCacheProxy hit in redis", "keyTemplate", p.keyTemplate, "id", id)
		cacheCounters.WithLabelValues(p.keyTemplate, "redis_hit").Inc()
		return true, err
	}
	if err == redis.Nil {
		redisMiss = true
		cacheCounters.WithLabelValues(p.keyTemplate, "redis_miss").Inc()
		common.Logger(ctx).Debugw("ObjectCacheProxy miss in redis", "keyTemplate", p.keyTemplate, "id", id)
	} else {
		//统计
		common.SysLogger().Errorw("ObjectCacheProxy redisGet error", "keyTemplate", p.keyTemplate, "err", err)
		common.Logger(ctx).Debugw("ObjectCacheProxy redisGet error", "keyTemplate", p.keyTemplate, "err", err)
	}

	//todo:防缓存击穿逻辑

	object, err := p.dataSource(ctx, id)
	if err != nil {
		//统计
		return false, err
	}
	if object == nil {
		//数据库中未命中
		//统计
		cacheCounters.WithLabelValues(p.keyTemplate, "db_empty").Inc()
		common.Logger(ctx).Debugw("ObjectCacheProxy miss in db", "keyTemplate", p.keyTemplate, "id", id)
		go p.saveToRedis(common.SetupContext(nil), id, NotExistsString)
		return false, nil
	}
	common.Logger(ctx).Debugw("ObjectCacheProxy hit in db", "keyTemplate", p.keyTemplate, "id", id)
	bytes, err := json.Marshal(object)
	if err != nil {
		return false, err
	}
	go p.saveToRedis(common.SetupContext(nil), id, bytes)
	err = json.Unmarshal(bytes, target)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (p *ObjectCacheProxy) saveToRedis(ctx context.Context, id interface{}, value interface{}) {
	cacheCounters.WithLabelValues(p.keyTemplate, "redis_set").Inc()
	_, err := p.redisClient.Set(ctx, p.assembleKey(id), value, p.expireTime).Result()
	if err != nil {
		//统计
		common.Logger(ctx).Errorw("ObjectCacheProxy saveToRedis error", "keyTemplate", p.keyTemplate, "id", id, "err", err)
	}
}
