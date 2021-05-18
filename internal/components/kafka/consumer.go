package kafka

import (
	"context"
	"github.com/Shopify/sarama"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"go.uber.org/zap"
	"sync"
	"time"
)

type (
	kafkaConsumer struct {
		key         string
		topic       string
		consumeFunc func(ctx context.Context, msg []byte)

		saramaConsumer sarama.ConsumerGroup
		setupReady     chan bool
		ctx            context.Context
		cancelRunning  context.CancelFunc
		wg             sync.WaitGroup
	}
)

var (
	_ sarama.ConsumerGroupHandler = new(kafkaConsumer)
)

func (consumer *kafkaConsumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(consumer.setupReady)
	return nil
}

func (consumer *kafkaConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (consumer *kafkaConsumer) close() {
	consumer.cancelRunning()
	consumer.wg.Wait()
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *kafkaConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	log.SysLogger().Infow("start consumeClaim", "topic", consumer.topic, "partition", claim.Partition(), "initialOffset", claim.InitialOffset())
	for message := range claim.Messages() {
		ctx := contexts.SetupContext(context.Background())
		contexts.AddLogFields(ctx, zap.String("entrance", consumer.key))
		contexts.AddLogFields(ctx, zap.String("entranceType", "Kafka"))
		contexts.Logger(ctx).Infow("receive Kafka msg", "msg", string(message.Value), "topic", message.Topic, "partition", message.Partition, "offset", message.Offset, "key", string(message.Key), "timestamp", message.Timestamp)
		consumer.consumeWithRecover(ctx, message.Value)
		session.MarkMessage(message, "")
	}
	return nil
}

func (consumer *kafkaConsumer) consumeWithRecover(ctx context.Context, bytes []byte) {
	defer func() {
		if err := recover(); err != nil {
			log.SysLogger().Warnw("kafka consumeFunc panic", "err", err, "msg", string(bytes))
		}
	}()
	consumer.consumeFunc(ctx, bytes)
}

func getKafkaConsumer(name string, consume func(ctx context.Context, msg []byte)) (*kafkaConsumer, error) {
	topic := kafkaConf.Topics[name].Topic
	group := kafkaConf.Topics[name].Group
	cluster := kafkaConf.Topics[name].Cluster

	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Consumer.Offsets.CommitInterval = 1 * time.Second
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	//consumerConfig.kafkaConsumer.Offsets.Initial = sarama.OffsetOldest
	consumerConfig.Consumer.Offsets.Retry.Max = 3
	consumerConfig.Consumer.Group.Session.Timeout = 30 * time.Second
	consumerConfig.Consumer.Group.Heartbeat.Interval = 3 * time.Second
	consumerConfig.Version = sarama.V1_0_0_0

	consumerGroup, err := sarama.NewConsumerGroup(kafkaConf.Clusters[cluster], group, consumerConfig)
	if err != nil {
		log.SysLogger().Errorf("Open consumer Failed! *Error is: %v", err)
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	consumer := kafkaConsumer{
		key:         name,
		topic:       topic,
		consumeFunc: consume,

		setupReady:     make(chan bool, 0),
		saramaConsumer: consumerGroup,
		ctx:            ctx,
		cancelRunning:  cancel,
	}
	return &consumer, nil
}

func (consumer *kafkaConsumer) start() {
	defer consumer.wg.Done()

	for {
		if err := consumer.saramaConsumer.Consume(consumer.ctx, []string{consumer.topic}, consumer); err != nil {
			log.SysLogger().Errorf("Error from consumer: %v, %v", consumer.topic, err)
		}
		// check if context was cancelled, signaling that the consumer should stop
		if consumer.ctx.Err() != nil {
			return
		}
		consumer.setupReady = make(chan bool, 0)
	}
}

func StartKafkaConsumer(name string, consumeFunc func(ctx context.Context, msg []byte)) {
	kafkaOnce.Do(loadKafkaConfig)
	kc, err := getKafkaConsumer(name, consumeFunc)
	if err != nil {
		log.SysLogger().Errorw("get kafka consumer error", "err", err)
		return
	}

	kc.wg.Add(1)
	go kc.start()

	<-kc.setupReady // Await till the consumer has been set up
	log.SysLogger().Infow("kafka consumer started", "topic", kc.topic)

	env.AddShutdownFunc(func() error {
		log.SysLogger().Infow("kafka consumer closing...", "topic", kc.topic)
		kc.close()
		if err = kc.saramaConsumer.Close(); err != nil {
			log.SysLogger().Errorw("closing kafka Consumer error", "topic", kc.topic, "error", err)
		} else {
			log.SysLogger().Infow("kafka consumer closed", "topic", kc.topic)
		}
		return nil
	})

	kc.wg.Wait()
}
