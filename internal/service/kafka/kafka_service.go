package kafka

import (
	"time"

	"github.com/afiff2/go-chat-server/internal/config"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/segmentio/kafka-go"
)

var KafkaService *kafkaService

type kafkaService struct {
	ChatWriter *kafka.Writer
	ChatReader *kafka.Reader
}

func init() {
	kafkaConfig := config.GetConfig().Kafka
	KafkaService = &kafkaService{}

	KafkaService.ChatWriter = &kafka.Writer{
		Addr:                   kafka.TCP(kafkaConfig.HostPort), //连接到的broker
		Topic:                  kafkaConfig.ChatTopic,
		Balancer:               &kafka.Hash{},
		WriteTimeout:           kafkaConfig.WriteTimeout * time.Second,
		RequiredAcks:           kafka.RequireOne, //只需 leader 分区确认
		AllowAutoTopicCreation: false,            //避免意外创建不必要的 topic
	}
	KafkaService.ChatReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaConfig.HostPort},
		Topic:          kafkaConfig.ChatTopic,
		CommitInterval: kafkaConfig.CommitTimeout * time.Second, //自动提交 offset 的间隔时间, Kafka 消费者会记录自己已经消费到了哪条消息（offset），方便故障恢复
		GroupID:        "chat",                                  //相同 GroupID 的多个消费者共同消费一个 topic，每个 partition 只能被组内一个消费者消费
		StartOffset:    kafka.LastOffset,                        //表示只消费新到达的消息，不会处理历史消息
	})
	if err := CreateTopic(); err != nil {
		zlog.Fatal("Kafka initialized failed.")
	} else {
		zlog.Info("Kafka initialized successfully.")
	}
}

func (k *kafkaService) KafkaClose() {
	if err := k.ChatWriter.Close(); err != nil {
		zlog.Error(err.Error())
	}
	if err := k.ChatReader.Close(); err != nil {
		zlog.Error(err.Error())
	}
}

// CreateTopic 创建topic
func CreateTopic() error {
	// 如果已经有topic了，就不创建了
	kafkaConfig := config.GetConfig().Kafka

	// 连接至任意kafka节点
	conn, err := kafka.Dial("tcp", kafkaConfig.HostPort) //使用 TCP 协议连接到一个 Kafka broker 节点
	if err != nil {
		zlog.Error(err.Error())
		return err
	}
	defer conn.Close()

	// 获取当前所有 topic 列表
	topicPartitions, err := conn.ReadPartitions()
	if err != nil {
		zlog.Error(err.Error())
		return err
	}

	// 判断目标 topic 是否已经存在
	topicExists := false
	for _, tp := range topicPartitions {
		if tp.Topic == kafkaConfig.ChatTopic {
			topicExists = true
			break
		}
	}

	//跟换topic设置时先手动删除原来的
	if !topicExists {
		topicConfigs := []kafka.TopicConfig{
			{
				Topic:             kafkaConfig.ChatTopic,
				NumPartitions:     kafkaConfig.Partition,
				ReplicationFactor: kafkaConfig.Replication,
			},
		}

		if err = conn.CreateTopics(topicConfigs...); err != nil {
			zlog.Error(err.Error())
			return err
		}
		zlog.Info("Kafka topic created.")
	}

	return nil
}
