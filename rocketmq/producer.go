package rocketmq

import (
	"context"
	"strings"
	"sync"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/apache/rocketmq-client-go/v2/rlog"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/log"
)

var (
	once      = sync.Once{}
	producers = make(map[string]*Producer)
)

type Producer struct {
	producer rocketmq.Producer
}

type MqConfig struct {
	Addrs string `json:"addrs"`
	// Retries        int           `json:"retries"`
	// GroupName      string        `json:"group_name"`
	InstanceName string `json:"instance_name"`
	// Namespace      string        `json:"namespace"`
	// SendMsgTimeout time.Duration `json:"send_msg_timeout"`
}

func (p Producer) SendSync(msg *primitive.Message) (*primitive.SendResult, error) {
	return p.producer.SendSync(context.Background(), msg)
}

func newProducer(mqConf MqConfig) *Producer {
	// var option []producer.Option

	p, err := rocketmq.NewProducer(
		producer.WithNsResolver(primitive.NewPassthroughResolver(strings.Split(mqConf.Addrs, ";"))),
		producer.WithQueueSelector(producer.NewHashQueueSelector()),
		// producer.WithRetry(mqConf.Retries),
		// producer.WithGroupName(mqConf.GroupName),
		producer.WithInstanceName(mqConf.InstanceName),
		// producer.WithNamespace(mqConf.Namespace),
		// producer.WithSendMsgTimeout(mqConf.SendMsgTimeout),
	)
	if err != nil {
		log.Fatalf("rocketmq NewProducer error %s:", err.Error())
		return nil
	}
	err = p.Start()
	if err != nil {
		log.Fatalf("rocketmq producer start error %s:", err.Error())
		return nil
	}
	return &Producer{p}
}

func GetProducer(key string) *Producer {
	once.Do(func() {
		initClient()
	})
	if p, ok := producers[key]; ok && p != nil {
		return p
	}
	return nil
}

func initClient() {
	rlog.SetLogger(baselog)
	var data map[string]MqConfig
	if err := config.Get("rocketmq").Scan(&data); err != nil {
		log.Fatal("error parsing rocketmq configuration file ", err)
		return
	}
	for key, conf := range data {
		producers[key] = newProducer(conf)
	}
}

func Close() {
	var err error
	for name, v := range producers {
		if v != nil {
			err = v.producer.Shutdown()
			if err != nil {
				log.Errorf("shutdown producer: %s error: %s", name, err.Error())
			}
		}
	}
}
