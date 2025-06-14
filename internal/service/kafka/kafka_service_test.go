package kafka

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

func TestKafkaInitialization(t *testing.T) {
	if KafkaService == nil {
		t.Fatal("Kafka service not initialized")
	}
	if KafkaService.ChatWriter == nil {
		t.Error("ChatWriter not initialized")
	}
	if KafkaService.ChatReader == nil {
		t.Error("ChatReader not initialized")
	}
}

func TestSendMessageAndReadMessage(t *testing.T) {
	defer KafkaService.KafkaClose()
	// 发送消息
	err := KafkaService.ChatWriter.WriteMessages(context.Background(), kafka.Message{
		Value: []byte("unit-test-message"),
	})
	assert.NoError(t, err)

	// 读取消息
	msg, err := KafkaService.ChatReader.ReadMessage(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "unit-test-message", string(msg.Value))
}
