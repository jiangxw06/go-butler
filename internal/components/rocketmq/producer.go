package rocketmq

import (
	"context"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/pkg/errors"
)

var (
	rocketMQCluster2Producer map[string]rocketmq.Producer
)

//经测试，目前rocketmq-client-go客户端不支持启动多个producer对应到不同的集群：只有最后启动的producer能找到topic
//暂时只支持同一个集群下的topic
func initRocketMQProducers() {
	rocketMQCluster2Producer = make(map[string]rocketmq.Producer)

	conf := sarama.NewConfig()
	conf.Producer.RequiredAcks = sarama.WaitForLocal
	conf.Producer.Return.Successes = true
	conf.Producer.Partitioner = sarama.NewHashPartitioner

	//为每个集群创建一个producer
	//for cluster, nameServer := range rocketMQCluster2NS {
	//	p, _ := rocketmq.NewProducer(
	//		producer.WithNameServerDomain(nameServer),
	//		producer.WithRetry(3),
	//	)
	//	err := p.Start()
	//	if err != nil {
	//		log.Error().Str("cluster",cluster).Msgf("Init RocketMQ producer error: %v",err)
	//		continue
	//	} else {
	//		log.Info().Str("cluster",cluster).Msgf("Init RocketMQ producer success!")
	//	}
	//
	//	go func() {
	//		signalChannel := make(chan os.Signal, 1)
	//		signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGQUIT)
	//		<-signalChannel
	//		if err := p.Shutdown(); err != nil {
	//			log.Error().Str("cluster",cluster).Msgf("shutdown RocketMQ producer error: %v",err)
	//		}
	//	}()
	//	rocketMQCluster2Producer[cluster] = p
	//}

	//只创建一个producer
	for _, topic := range rocketMQConf.Topics {
		cluster := topic.Cluster
		if cluster == "" {
			continue
		}
		nameServer := topic.NameServer
		p, _ := rocketmq.NewProducer(
			producer.WithNameServerDomain(nameServer),
			producer.WithRetry(3),
		)
		err := p.Start()
		if err != nil {
			log.SysLogger().Errorw("Init RocketMQ producer error", "cluster", cluster, "error", err)
			continue
		} else {
			log.SysLogger().Infow("Init RocketMQ producer success!", "cluster", cluster)
		}

		env.AddShutdownFunc(func() error {
			log.SysLogger().Infow("RocketMQ producer closing...", "cluster", cluster)
			if err := p.Shutdown(); err != nil {
				log.SysLogger().Errorw("shutdown RocketMQ producer error", "cluster", cluster, "error", err)
			} else {
				log.SysLogger().Infow("RocketMQ producer closed", "cluster", cluster)
			}
			return nil
		})

		rocketMQCluster2Producer[cluster] = p
		break
	}
}

func SendRocketMQMessage(ctx context.Context, name, key string, data []byte) (err error) {
	rocketOnce.Do(loadRocketMQConfig)
	defer func() {
		if pan := recover(); pan != nil {
			log.SysLogger().Errorw("send RocketMQ msg panic", "key", key, "panic", pan, "name", name)
			err = errors.New(fmt.Sprintf("%v", pan))
		}
		return
	}()

	topic := rocketMQConf.Topics[name].Topic
	cluster := rocketMQConf.Topics[name].Cluster
	p := rocketMQCluster2Producer[cluster]

	msg := &primitive.Message{
		Topic: topic,
		Body:  data,
	}
	msg = msg.WithKeys([]string{key})
	res, err := p.SendSync(context.Background(), msg)

	if err != nil {
		contexts.Logger(ctx).Errorw("send RocketMQ msg error", "topic", topic, "key", key, "data", string(data), "err", err)
		return errors.Errorf("send RocketMQ msg err")
	} else if res.Status != primitive.SendOK {
		contexts.Logger(ctx).Errorw("send RocketMQ msg error", "topic", topic, "key", key, "data", string(data), "res.Status", res.Status)
		return errors.Errorf("send RocketMQ msg failed")
	} else {
		contexts.Logger(ctx).Debugw("send RocketMQ msg", "topic", topic, "MsgID", res.MsgID, "key", key, "data", string(data), "MessageQueue", res.MessageQueue.String(), "offset", res.QueueOffset)
	}
	return
}
