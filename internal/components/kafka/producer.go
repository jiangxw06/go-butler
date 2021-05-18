package kafka

import (
	"context"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/pkg/errors"
	"strings"
	"time"
)

var (
	kafkaCluster2Producer map[string]sarama.SyncProducer
)

func initProducers() {
	kafkaCluster2Producer = make(map[string]sarama.SyncProducer)

	for cluster, brokers := range kafkaConf.Clusters {
		p, err := newProducer(cluster, brokers)
		if err != nil {
			log.SysLogger().Errorw("Init kafka producer error", "cluster", cluster, "error", err)
			continue
		}
		log.SysLogger().Infow("Init kafka producer success", "cluster", cluster)
		kafkaCluster2Producer[cluster] = p
	}
}

func newProducer(cluster string, brokers []string) (sarama.SyncProducer, error) {
	conf := sarama.NewConfig()
	conf.Producer.RequiredAcks = sarama.WaitForLocal
	conf.Producer.Return.Successes = true
	conf.Producer.Partitioner = sarama.NewHashPartitioner
	conf.Net.DialTimeout = 1000 * time.Millisecond
	conf.Net.ReadTimeout = 1000 * time.Millisecond
	conf.Net.WriteTimeout = 1000 * time.Millisecond

	p, err := sarama.NewSyncProducer(brokers, conf)
	if err != nil {
		return nil, err
	}
	env.AddShutdownFunc(func() error {
		log.SysLogger().Infow("kafka producer closing...", "cluster", cluster)
		if err := p.Close(); err != nil {
			log.SysLogger().Errorw("close kafka producer error", "cluster", cluster, "brokers", brokers, "error", err)
		} else {
			log.SysLogger().Infow("kafka producer closed", "cluster", cluster)
		}
		return nil
	})
	return p, nil
}

func SendKafkaMessage(ctx context.Context, name, key string, data []byte) (err error) {
	kafkaOnce.Do(loadKafkaConfig)
	defer func() {
		if pan := recover(); pan != nil {
			log.SysLogger().Errorw("send kafka msg panic", "name", name, "key", key, "panic", pan)
			err = errors.New(fmt.Sprintf("%v", pan))
		}
		return
	}()

	topic := kafkaConf.Topics[name].Topic
	cluster := kafkaConf.Topics[name].Cluster
	producer := kafkaCluster2Producer[cluster]

	var msgKey sarama.Encoder = nil
	if key != "" {
		msgKey = sarama.StringEncoder(strings.TrimSpace(key))
	}
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   msgKey,
		Value: sarama.ByteEncoder(data),
	}
	partition, offset, err := producer.SendMessage(msg)
	if err != nil {
		contexts.Logger(ctx).Errorw("send kafka msg err", "topic", topic, "key", key, "data", string(data), "err", err)
		return errors.Errorf("send kafka msg err")
	} else {
		contexts.Logger(ctx).Debugw("send kafka msg", "topic", topic, "partition", partition, "offset", offset, "data", string(data))
	}
	return
}
