package rocketmq

import (
	"context"
	"flag"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/magiconair/properties/assert"
	"os"
	"testing"
)

var (
	ctx context.Context
)

func TestMain(m *testing.M) {
	env.MergeConfig(`
[rocketmq]
    #使用到的各个topic
    #xiaohuadasai为该topic的name
    [rocketmq.topics.xiaohuadasai]
        cluster = "test"
        topic = "pugc-addfans-from-activity-test-topic"
        group = "ugc-addfans-from-activity-test-consumer"

`)

	flag.Parse()
	ctx = contexts.SetupContext(nil)
	exitCode := m.Run()

	// 退出
	os.Exit(exitCode)
}

func TestRocketMQProducer(t *testing.T) {
	bytes := []byte("{\"activityId\":15,\"userId\":210745706,\"createTime\":1614842379759}")
	err := SendRocketMQMessage(ctx, "xiaohuadasai", "testKey", bytes)
	assert.Equal(t, err, nil)
	env.WaitClose()
}

func TestRocketMQConsumer(t *testing.T) {
	go StartRocketMQConsumer("xiaohuadasai", Consume)
	env.WaitClose()
}

func Consume(ctx context.Context, data []byte) bool {
	contexts.Logger(ctx).Infow("", "msg", string(data))
	return true
}
