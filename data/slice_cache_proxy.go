package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jiangxw06/go-butler"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"math"
	"time"
)

const (
	//HighToLowOrder 指ListGroup的元素按score从高到低排序
	HighToLowOrder RangeOrderType = iota
	//LowToHighOrder 指ListGroup的元素按score从低到高排序
	LowToHighOrder
)

var (
	lowestMember = redis.Z{
		Score:  math.Inf(-1),
		Member: "",
	}
	highestZMember = redis.Z{
		Score:  math.Inf(1),
		Member: "",
	}
)

type (
	//RangeOrderType 是ListGroup的元素按照score的高低排序的方式
	RangeOrderType int

	//RangeInfo 是查询zset以及数据源的参数，用于service层查询缓存，也用于缓存在miss时查询数据源
	//OrderBy 是本次查询的score排序方式，仅用于缓存层查询数据源时（service层调用RangeByScore/RangeByRank是传入的OrderBy被忽略）
	//StartScore 仅当RangeBy=RangeByScoreType时有用，是上次查询的最后一个对象的score，若第一次查询应该传入math.Inf(1)或math.Inf(-1)，取决于ListGroup的OrderBy类型
	//IncludeStartScore 仅当RangeBy=RangeByScoreType时有用，表示结果集中是否应该包含参数score，一般的分页查询应该传入false，表示不会包含上次查询的最后一个对象
	//Limit 表示返回元素个数的上限
	RangeInfo struct {
		StartScore        float64
		FromBeginning     bool
		IncludeStartScore bool //若按Score查询，结果集是否包含边界值Score
		Offset            int64
		Limit             int64
	}

	//SliceCacheProxy 是有相同业务逻辑的一组zset对象的集合
	SliceCacheProxy struct {
		keyTemplate string
		expireTime  time.Duration //设置缓存过期时间，为0则永不过期
		redisClient redis.UniversalClient
		//Range获取member时，从score最高的开始还是从最低的开始
		scoreOrder RangeOrderType
		//缓存列表最大长度
		CacheCapacity int64
		idMapping     func(daoMutation DaoMutation) (id interface{})
		dataSource    func(ctx context.Context, id interface{}, info RangeInfo) (members []*redis.Z, err error)

		//实现DaoEventListener，异步删除陈旧的缓存key
		daoEventChan        chan DaoMutation
		daoEventChanClosed  bool
		cleanerClosed       chan struct{}
		asyncDelKeyInterval time.Duration
	}
)

var (
	_ DaoEventListener = new(ObjectCacheProxy)
)

func (p *SliceCacheProxy) Listen(event DaoMutation) {
	cacheCounters.WithLabelValues(p.keyTemplate, "db_mutation").Inc()
	if p.daoEventChanClosed {
		common.SysLogger().Warnw("receive daoMutation after closing objectCacheProxy chan", "keyTemplate", p.keyTemplate, "event", event)
	} else {
		p.daoEventChan <- event
	}
}

//NewListGroup 返回一个ListGroup对象
func NewSliceCacheProxy(redisClient redis.UniversalClient, keyTemplate string) *SliceCacheProxy {
	cache := &SliceCacheProxy{}
	cache.keyTemplate = keyTemplate
	cache.redisClient = redisClient
	cache.scoreOrder = HighToLowOrder
	cache.CacheCapacity = 1000
	cache.expireTime = time.Hour

	cache.daoEventChan = make(chan DaoMutation, 1000)
	cache.asyncDelKeyInterval = time.Millisecond * 10
	cache.cleanerClosed = make(chan struct{})

	env.AddShutdownFunc(func() error {
		common.SysLogger().Infow("closing sliceCacheProxy", "keyTemplate", cache.keyTemplate)
		//关闭daoEvent通道
		cache.daoEventChanClosed = true
		close(cache.daoEventChan)
		//等待cleaner结束
		<-cache.cleanerClosed
		common.SysLogger().Infow("sliceCacheProxy closed", "keyTemplate", cache.keyTemplate)
		return nil
	})

	go cache.startStaleKeyCleaner()

	return cache
}

//SetExpiration 设置过期时间
//expireType 是更新过期时间的方式
//expireType = ManuallyExpirationType(默认)，表示用户自己管理过期时间
//expireType = AfterWriteExpirationType, 表示string对象在被set后更新过期时间
//expireType = AfterReadExpirationType，表示string对象在被访问后更新过期时间
//expireType = AfterReadAndWriteExpirationType, 表示string对象在被get/set后更新过期时间
func (p *SliceCacheProxy) SetExpiration(expiration time.Duration) *SliceCacheProxy {
	p.expireTime = expiration
	return p
}

//SetScoreOrder 设置zset中元素排序的方式
func (p *SliceCacheProxy) SetCacheCapacity(cap int64) *SliceCacheProxy {
	p.CacheCapacity = cap
	return p
}

//SetScoreOrder 设置zset中元素排序的方式
func (p *SliceCacheProxy) SetRangeOrder(order RangeOrderType) *SliceCacheProxy {
	p.scoreOrder = order
	return p
}

//SetDataSource 设置从数据源查询数据的函数
func (p *SliceCacheProxy) SetDataSource(f func(ctx context.Context, id interface{}, info RangeInfo) (members []*redis.Z, err error)) *SliceCacheProxy {
	p.dataSource = f
	return p
}

func (p *SliceCacheProxy) SetStaleKeyCleaner(publisher DaoEventPublisher, mapping func(daoMutation DaoMutation) (id interface{})) *SliceCacheProxy {
	publisher.AddListener(p)
	p.idMapping = mapping
	return p
}

func (p *SliceCacheProxy) getFinalZMember() redis.Z {
	if p.scoreOrder == HighToLowOrder {
		return lowestMember
	}
	return highestZMember
}

func (p *SliceCacheProxy) assembleKey(id interface{}) (key string) {
	key = fmt.Sprintf(p.keyTemplate, id)
	return
}

func (p *SliceCacheProxy) delKeys(ids []interface{}) {
	cacheCounters.WithLabelValues(p.keyTemplate, "redis_del").Inc()
	pipe := p.redisClient.Pipeline()
	for _, id := range ids {
		key := p.assembleKey(id)
		pipe.Del(context.Background(), key)
	}
	cmders, err := pipe.Exec(context.Background())
	if err != nil {
		log.SysLogger().Errorw("SliceCacheProxy delete key error", "keyTemplate", p.keyTemplate, "ids", ids, "err", err)
		return
	}
	var delCounts int64
	for _, cmder := range cmders {
		cmd := cmder.(*redis.IntCmd)
		delCount, err := cmd.Result()
		if err != nil {
			//按我的理解应该不会进来
			log.SysLogger().Errorw("SliceCacheProxy err which shouldn't enter", "keyTemplate", p.keyTemplate, "ids", ids, "err", err)
		}
		delCounts += delCount
	}
	log.SysLogger().Debugw("SliceCacheProxy delKeys", "keyTemplate", p.keyTemplate, "len(ids)", len(ids), "delCounts", delCounts)
}

func (p *SliceCacheProxy) startStaleKeyCleaner() {
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

//RangeByScore 按照Score查询列表元素，排序方式是初始化时指定的ScoreOrder
//score是范围的起始score
//inclusive 为true表示结果集包含边界值score
func (p *SliceCacheProxy) Get(ctx context.Context, id interface{}, info RangeInfo, value interface{}) (err error) {
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

	common.Logger(ctx).Debugw("SliceCacheProxy Get begin", "keyTemplate", p.keyTemplate, "id", id)
	if info.Limit < 1 {
		info.Limit = 1
	}
	var start string
	if info.FromBeginning {
		cacheCounters.WithLabelValues(p.keyTemplate, "get_from_beginning").Inc()
		if p.scoreOrder == HighToLowOrder {
			start = "+inf"
		} else {
			start = "-inf"
		}
	} else {
		start = fmt.Sprintf("%v", info.StartScore)
	}
	if !info.IncludeStartScore {
		start = "(" + start
	}
	var min, max string
	if p.scoreOrder == HighToLowOrder {
		max = start
		min = "-inf"
	} else {
		min = start
		max = "+inf"
	}
	rangeBy := redis.ZRangeBy{
		Min:    min,
		Max:    max,
		Offset: info.Offset,
		Count:  info.Limit,
	}
	var res []redis.Z
	if p.scoreOrder == HighToLowOrder {
		res, err = p.redisClient.ZRevRangeByScoreWithScores(ctx, p.assembleKey(id), &rangeBy).Result()
	} else {
		res, err = p.redisClient.ZRangeByScoreWithScores(ctx, p.assembleKey(id), &rangeBy).Result()
	}
	if err != nil {
		common.SysLogger().Errorw("SliceCacheProxy redisGet error", "keyTemplate", p.keyTemplate, "err", err)
		common.Logger(ctx).Debugw("SliceCacheProxy redisGet error", "keyTemplate", p.keyTemplate, "err", err)

		//查Redis出错的话则从source中get
		redisMiss = true
		return p.GetFromSource(ctx, id, info, value)
	}

	//获取真实res和可能的final标志
	var hasFinal bool
	if len(res) > 0 && res[len(res)-1] == p.getFinalZMember() {
		res = res[:len(res)-1]
		hasFinal = true
	}

	//直接从Redis数据get的情况
	if len(res) == int(info.Limit) || //已经获得足够数据
		hasFinal { //db中也没有更多数据
		cacheCounters.WithLabelValues(p.keyTemplate, "redis_hit").Inc()
		common.Logger(ctx).Debugw("SliceCacheProxy Get hit in Redis", "keyTemplate", p.keyTemplate, "id", id, "len(data)", len(res))
		if len(res) == 0 {
			//没有命中的数据
			cacheCounters.WithLabelValues(p.keyTemplate, "redis_empty").Inc()
			return
		}

		//拼接数组形式的json字符串
		var buffer bytes.Buffer
		buffer.WriteString("[")
		for i := 0; i < len(res)-1; i++ {
			buffer.WriteString(res[i].Member.(string))
			buffer.WriteString(",")
		}
		buffer.WriteString(res[len(res)-1].Member.(string))
		buffer.WriteString("]")
		err = json.Unmarshal(buffer.Bytes(), value)
		if err != nil {
			//反序列化失败
			return err
		}
		//成功
		return
	}
	redisMiss = true
	cacheCounters.WithLabelValues(p.keyTemplate, "redis_miss").Inc()

	//异步：查redis中本键是否存在，不存在，则reload前N条到redis中
	go p.reloadRedisIfNotExists(ctx, id)

	//查source，fill进value
	return p.GetFromSource(ctx, id, info, value)
}

func (p *SliceCacheProxy) reloadRedisIfNotExists(ctx context.Context, id interface{}) {
	cacheCounters.WithLabelValues(p.keyTemplate, "redis_exists").Inc()
	existCount, err := p.redisClient.Exists(ctx, p.assembleKey(id)).Result()
	if err != nil {
		common.SysLogger().Errorw("SliceCacheProxy Get exists", "keyTemplate", p.keyTemplate, "err", err)
		return
	}
	if existCount == 1 {
		//缓存存在
	} else {
		p.reloadRedis(ctx, id)
	}
}

func (p *SliceCacheProxy) reloadRedis(ctx context.Context, id interface{}) {
	cacheCounters.WithLabelValues(p.keyTemplate, "redis_zadd").Inc()
	zmembers, err := p.dataSource(ctx, id, RangeInfo{
		FromBeginning: true,
		Offset:        0,
		Limit:         p.CacheCapacity,
	})
	if err != nil {
		common.SysLogger().Errorw("SliceCacheProxy reloadRedis dataSource error", "err", err, "keyTemplate", p.keyTemplate, "id", id)
		return
	}

	//注意：redis中的member都是json序列化后的值
	for _, zmember := range zmembers {
		zmember.Member, _ = json.Marshal(zmember.Member)
	}

	//增加Final标志位
	var reachFinal bool
	if len(zmembers) < int(p.CacheCapacity) {
		reachFinal = true
		finalZMember := p.getFinalZMember()
		zmembers = append(zmembers, &finalZMember)
	}

	key := p.assembleKey(id)
	addCount, err := p.redisClient.ZAdd(ctx, key, zmembers...).Result()
	if err != nil {
		common.SysLogger().Errorw("SliceCacheProxy reloadRedis zadd error", "err", err, "keyTemplate", p.keyTemplate, "id", id, "reachFinal", reachFinal)
	} else {
		common.SysLogger().Debugw("SliceCacheProxy reloadRedis", "addCount", addCount, "keyTemplate", p.keyTemplate, "id", id, "reachFinal", reachFinal)
	}
	success, err := p.redisClient.Expire(ctx, key, p.expireTime).Result()
	if err != nil {
		common.SysLogger().Errorw("SliceCacheProxy reloadRedis expire error", "err", err, "keyTemplate", p.keyTemplate, "id", id)
	} else {
		if success {
			//统计
		}
	}
}

func (p *SliceCacheProxy) GetFromSource(ctx context.Context, id interface{}, info RangeInfo, value interface{}) (err error) {
	//cacheCounters.WithLabelValues(p.keyTemplate, "db_get").Inc()
	zmembers, err := p.dataSource(ctx, id, info)
	common.Logger(ctx).Debugw("SliceCacheProxy GetFromSource", "err", err, "keyTemplate", p.keyTemplate, "id", id)
	if err != nil {
		return err
	}
	var members []interface{}
	for _, member := range zmembers {
		members = append(members, member.Member)
	}
	if len(members) == 0 {
		cacheCounters.WithLabelValues(p.keyTemplate, "db_empty").Inc()
		return nil
	}
	jsonStr, _ := json.Marshal(members)
	return json.Unmarshal(jsonStr, value)
}
