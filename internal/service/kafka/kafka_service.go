package kafka

import (
	"time"

	"github.com/afiff2/go-chat-server/internal/config"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

var KafkaService *kafkaService

type kafkaService struct {
	ChatWriter *kafka.Writer
	ChatReader *kafka.Reader
}

func waitForTopic(broker, topic string) {
	for i := 0; i < 10; i++ {
		conn, err := kafka.Dial("tcp", broker)
		if err == nil {
			parts, _ := conn.ReadPartitions(topic)
			conn.Close()
			if len(parts) > 0 {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	zlog.Warn("等待 Kafka topic 元数据超时，可能还读不到分区", zap.String("topic", topic))
}

func init() {
	kafkaConfig := config.GetConfig().Kafka

	if err := RecreateTopic(); err != nil {
		zlog.Fatal("Kafka initialized failed.")
	} else {
		zlog.Info("Kafka initialized successfully.")
	}

	zlog.Info("Kafka topic 已创建，开始等待 metadata 生效")
	waitForTopic(kafkaConfig.HostPort, kafkaConfig.ChatTopic)

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
}

func (k *kafkaService) Close() {
	if err := k.ChatWriter.Close(); err != nil {
		zlog.Error("关闭 Kafka 写入器失败", zap.Error(err))
	}
	if err := k.ChatReader.Close(); err != nil {
		zlog.Error("关闭 Kafka 读取器失败", zap.Error(err))
	}
	zlog.Info("Kafka 连接已成功关闭")
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

// RecreateTopic 先删除再创建 chat topic
func RecreateTopic() error {
	kafkaConfig := config.GetConfig().Kafka

	// 1. 连接到任意一个 broker
	conn, err := kafka.Dial("tcp", kafkaConfig.HostPort)
	if err != nil {
		zlog.Error("dial kafka 失败", zap.Error(err))
		return err
	}
	defer conn.Close()

	// 2. 删除旧的 topic
	if err := conn.DeleteTopics(kafkaConfig.ChatTopic); err != nil {
		// 如果删除失败，可以选择打印警告，但不一定要 return
		zlog.Warn("删除 Kafka topic 失败（可能不存在）", zap.Error(err))
	} else {
		zlog.Info("已删除旧的 Kafka topic", zap.String("topic", kafkaConfig.ChatTopic))
	}

	// 3. 创建新的 topic
	topicConfigs := []kafka.TopicConfig{{
		Topic:             kafkaConfig.ChatTopic,
		NumPartitions:     kafkaConfig.Partition,
		ReplicationFactor: kafkaConfig.Replication,
	}}
	if err := conn.CreateTopics(topicConfigs...); err != nil {
		zlog.Error("创建 Kafka topic 失败", zap.Error(err))
		return err
	}
	zlog.Info("已创建新的 Kafka topic", zap.String("topic", kafkaConfig.ChatTopic))
	return nil
}
