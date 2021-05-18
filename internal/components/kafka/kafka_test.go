package kafka

import (
	"context"
	"flag"
	"fmt"
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
[kafka]
    #kafka集群配置，默认是土城集群
    [kafka.clusters]
        default = ["www.mybroker1.com"]
    #使用到的各个topic
    [kafka.topics.my_topic_name]
        #cluster = "default" #default集群可以不配
        topic = "my-topic"
        group = "my-group"
`)

	flag.Parse()
	ctx = contexts.SetupContext(nil)
	exitCode := m.Run()

	env.CloseComponents()
	// 退出
	os.Exit(exitCode)
}

func TestSendKafkaMsg(t *testing.T) {
	err := SendKafkaMessage(ctx, "my_topic_name", "test_j", []byte("mymsg"))
	assert.Equal(t, err, nil)
	env.WaitClose()
}

func TestKafkaConsumer(t *testing.T) {
	go StartKafkaConsumer("my_topic_name", consume)
	env.WaitClose()
}

func consume(ctx2 context.Context, msg []byte) {
	fmt.Printf("msg: %v", string(msg))
}
