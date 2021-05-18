// This package provides unique id in distribute system
// the algorithm is inspired by Twitter's famous snowflake
// its link is: https://github.com/twitter/snowflake/releases/tag/snowflake-2010
//

// +---------------+----------------+----------------+
// |timestamp(ms)42  | worker id(10) | sequence(12)	 |
// +---------------+----------------+----------------+

// Copyright (C) 2016 by zheng-ji.info

package data

import (
	"context"
	"errors"
	"fmt"
	"github.com/jiangxw06/go-butler"
	"github.com/jiangxw06/go-butler/internal/env"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"sync"
	"time"
)

const (
	cEpoch        = 1483200000000 // 2017-01-01 00:00:00
	cWorkerIDBits = 10            // Num of WorkerId Bits
	cSequenceBits = 12            // Num of Sequence Bits

	cWorkerIDShift  = 12
	cTimeStampShift = 22

	cSequenceMask = 0xfff // equal as getSequenceMask()
	cMaxWorker    = 0x3ff // equal as getMaxWorkerId()
)

// idGenerator Struct
type idGenerator struct {
	workerID      int64
	lastTimeStamp int64
	sequence      int64
	maxWorkerID   int64
	lock          *sync.Mutex
}

var (
	idGenerator0  *idGenerator
	snowflakeOnce sync.Once
)

func initIDGeneratorByRedis() {
	redisCli := common.MustGetRedisClient("snowflake")
	workerID, err := redisCli.Incr(context.Background(), fmt.Sprintf("app:%v:nextworkerid", env.GetAppName())).Result()
	if err != nil {
		panic(err)
	}
	workerID = workerID % getMaxWorkerID()
	idGenerator0 = newIDGenerator(workerID)
}

func InitIDGenerator(workerID int64) {
	snowflakeOnce.Do(func() {
		idGenerator0 = newIDGenerator(workerID)
	})
}

// newIDGenerator Func: Generate NewIdWorker with Given workerid
func newIDGenerator(workerID int64) (iw *idGenerator) {
	iw = new(idGenerator)

	iw.maxWorkerID = getMaxWorkerID()

	iw.workerID = workerID
	iw.lastTimeStamp = -1
	iw.sequence = 0
	iw.lock = new(sync.Mutex)
	return iw
}

func getMaxWorkerID() int64 {
	return -1 ^ -1<<cWorkerIDBits
}

func getSequenceMask() int64 {
	return -1 ^ -1<<cSequenceBits
}

// return in ms
func (ig *idGenerator) timeGen() int64 {
	return time.Now().UnixNano() / 1000 / 1000
}

func (ig *idGenerator) timeReGen(last int64) int64 {
	ts := time.Now().UnixNano() / 1000 / 1000
	for {
		if ts <= last {
			ts = ig.timeGen()
		} else {
			break
		}
	}
	return ts
}

func NextSnowflakeID() (ts int64, err error) {
	snowflakeOnce.Do(initIDGeneratorByRedis)
	if idGenerator0 == nil {
		return 0, errors.New("id generator not initialized")
	}
	return idGenerator0.nextSnowflakeID()
}

// NewID Func: Generate next id
func (ig *idGenerator) nextSnowflakeID() (ts int64, err error) {
	ig.lock.Lock()
	defer ig.lock.Unlock()
	ts = ig.timeGen()
	if ts == ig.lastTimeStamp {
		ig.sequence = (ig.sequence + 1) & cSequenceMask
		if ig.sequence == 0 {
			ts = ig.timeReGen(ts)
		}
	} else {
		ig.sequence = 0
	}

	if ts < ig.lastTimeStamp {
		err = errors.New("clock moved backwards, Refuse gen id")
		return 0, err
	}
	ig.lastTimeStamp = ts
	ts = (ts-cEpoch)<<cTimeStampShift | ig.workerID<<cWorkerIDShift | ig.sequence
	return ts, nil
}

// ParseSnowflakeID Func: reverse uid to timestamp, workid, seq
func ParseSnowflakeID(id int64) (t time.Time, ts int64, workerID int64, seq int64) {
	seq = id & cSequenceMask
	workerID = (id >> cWorkerIDShift) & cMaxWorker
	ts = (id >> cTimeStampShift) + cEpoch
	t = time.Unix(ts/1000, (ts%1000)*1000000)
	return
}
