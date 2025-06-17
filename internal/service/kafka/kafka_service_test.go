package kafka

import (
	"context"
	"testing"
	"time"

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
	defer KafkaService.Close()
	ctx := context.Background()

	// 启动一个 goroutine 先读消息
	msgCh := make(chan kafka.Message, 1)
	errCh := make(chan error, 1)
	go func() {
		msg, err := KafkaService.ChatReader.ReadMessage(ctx)
		if err != nil {
			errCh <- err
			return
		}
		msgCh <- msg
	}()

	// 让 Reader 稍微“就绪”
	time.Sleep(100 * time.Millisecond)

	// 再发送消息
	err := KafkaService.ChatWriter.WriteMessages(ctx, kafka.Message{
		Value: []byte("unit-test-message"),
	})
	assert.NoError(t, err)

	// 等待读取结果，给个超时以防万一
	select {
	case msg := <-msgCh:
		assert.Equal(t, "unit-test-message", string(msg.Value))
	case err := <-errCh:
		t.Fatalf("read failed: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}
