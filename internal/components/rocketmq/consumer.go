package rocketmq

import (
	"context"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"go.uber.org/zap"
)

type (
	myConsumer struct {
		name        string
		topic       *topic
		consumeFunc func(ctx context.Context, msg []byte) bool
	}
)

func (c *myConsumer) Consume(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	ctx = contexts.SetupContext(ctx)
	contexts.AddLogFields(ctx, zap.String("entrance", c.name))
	contexts.AddLogFields(ctx, zap.String("entranceType", "RocketMQ"))
	for _, msg := range msgs {
		contexts.Logger(ctx).Debugw("receive RocketMQ msg", "msg", string(msg.Body),
			"topic", msg.Topic, "properties", msg.GetProperties(), "msgId", msg.MsgId, "offsetMsgId", msg.OffsetMsgId, "size", msg.StoreSize)
		_ = c.consumeFunc(ctx, msg.Body)
	}
	return consumer.ConsumeSuccess, nil
}

func StartRocketMQConsumer(name string, consumeFunc func(ctx context.Context, msg []byte) bool) {
	rocketOnce.Do(loadRocketMQConfig)
	topic := rocketMQConf.Topics[name]
	nameServer := rocketMQCluster2NS[topic.Cluster]

	mc := &myConsumer{
		name:        name,
		topic:       topic,
		consumeFunc: consumeFunc,
	}

	c, _ := rocketmq.NewPushConsumer(
		consumer.WithGroupName(topic.Group),
		consumer.WithNameServerDomain(nameServer),
	)
	err := c.Subscribe(topic.Topic, consumer.MessageSelector{}, mc.Consume)
	if err != nil {
		log.SysLogger().Errorw("subscribe RocketMQ topic error", "topic", name, "error", err)
		return
	}
	err = c.Start()
	if err != nil {
		log.SysLogger().Errorw("start RocketMQ consumer error", "topic", name, "error", err)
		return
	} else {
		log.SysLogger().Infow("Init RocketMQ consumer success!", "topic", name)
	}

	env.AddShutdownFunc(func() error {
		log.SysLogger().Infow("RocketMQ consumer closing...", "topic", name)
		if err := c.Shutdown(); err != nil {
			log.SysLogger().Errorw("shutdown RocketMQ consumer error", "topic", name, "error", err)
		} else {
			log.SysLogger().Infow("RocketMQ consumer closed", "topic", name)
		}
		return nil
	})

}
