package rocketmq

import (
	"github.com/apache/rocketmq-client-go/v2/rlog"
	"go.uber.org/zap/zapcore"
	"strings"
	"sync"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/log"
)

var (
	onceInitPushConsumer = sync.Once{}
	pushConsumer         = make(map[string]rocketmq.PushConsumer)
)

type PushConsumetConfig struct {
	Addrs                      string `json:"addrs"`
	ConsumeMessageBatchMaxSize int    `json:"consume_message_batch_max_size"`
	BatchSize                  int32  `json:"batch_size"`
	GroupName                  string `json:"group_name"`
	Order                      bool   `json:"order"`
	Retries                    int32  `json:"retries"`
	// GroupName      string        `json:"group_name"`
	InstanceName string `json:"instance_name"`
	// Namespace      string        `json:"namespace"`
	// SendMsgTimeout time.Duration `json:"send_msg_timeout"`
}

func GetPushConsumer(key string) rocketmq.PushConsumer {
	onceInitPushConsumer.Do(func() {
		initConsumerClient(baselog)
	})
	if p, ok := pushConsumer[key]; ok && p != nil {
		return p
	}
	return nil
}

func GetPushConsumerWithLogLevel(key string, level zapcore.Level) rocketmq.PushConsumer {
	onceInitPushConsumer.Do(func() {
		initConsumerClient(&Log{
			LogLevel: level,
			Logger:   log.Logger(),
		})
	})
	if p, ok := pushConsumer[key]; ok && p != nil {
		return p
	}
	return nil
}

func NewPushConsumet(mqConf PushConsumetConfig) rocketmq.PushConsumer {
	c, _ := rocketmq.NewPushConsumer(
		consumer.WithNsResolver(primitive.NewPassthroughResolver(strings.Split(mqConf.Addrs, ";"))),
		consumer.WithPullBatchSize(mqConf.BatchSize),
		consumer.WithConsumeMessageBatchMaxSize(mqConf.ConsumeMessageBatchMaxSize),
		consumer.WithConsumerOrder(mqConf.Order),
		consumer.WithConsumerModel(consumer.Clustering),
		consumer.WithGroupName(mqConf.GroupName),
		consumer.WithMaxReconsumeTimes(mqConf.Retries),
		consumer.WithInstance(mqConf.InstanceName))

	return c
}

func initConsumerClient(l *Log) {
	rlog.SetLogger(l)
	var data map[string]PushConsumetConfig
	if err := config.Get("rocketmq").Scan(&data); err != nil {
		log.Fatal("error parsing rocketmq configuration file ", err)
		return
	}
	for key, conf := range data {
		pushConsumer[key] = NewPushConsumet(conf)
	}
}
