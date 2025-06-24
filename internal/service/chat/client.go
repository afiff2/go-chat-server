package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/internal/model"
	myKafka "github.com/afiff2/go-chat-server/internal/service/kafka"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/enum/message/message_status_enum"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type MessageBack struct {
	Message []byte
	Uuid    string
}

type Client struct {
	Conn       *websocket.Conn
	Uuid       string
	SendBack   chan *MessageBack // 给前端
	closeOnce  sync.Once         // 确保资源只关闭一次
	writeMutex sync.Mutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
	// 检查连接的Origin头
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// send 封装了对 Conn.WriteMessage 的并发保护
func (c *Client) send(msgType int, data []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return c.Conn.WriteMessage(msgType, data)
}

// NewClientInit 当接受到前端有登录消息时，会调用该函数
func NewClientInit(c *gin.Context, clientId string) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Error("upgrade websocket failed", zap.Error(err))
		return
	}
	client := &Client{
		Conn:     conn,
		Uuid:     clientId,
		SendBack: make(chan *MessageBack, constants.CHANNEL_SIZE),
	}

	KafkaChatServer.AddClient(client)
	zlog.Info(fmt.Sprintf("用户%s登录\n", client.Uuid))
	err = client.send(websocket.TextMessage, []byte("欢迎来到聊天服务器😊"))
	if err != nil {
		zlog.Error(err.Error())
	}

	go client.readLoop()
	go client.writeLoop()
	zlog.Info("ws 连接成功", zap.String("uuid", clientId))
}

// 关闭逻辑
func (c *Client) close() {
	c.closeOnce.Do(func() {
		//从map中移除
		KafkaChatServer.RemoveClient(c.Uuid)
		//清理资源
		_ = c.Conn.Close()
		close(c.SendBack)
	})
}

// ClientLogout 当接受到前端有登出消息时，会调用该函数
func ClientLogout(clientId string) (string, int) {
	client, _ := KafkaChatServer.GetClient(clientId)
	if client != nil {
		zlog.Info(fmt.Sprintf("用户%s退出登录\n", client.Uuid))
		if err := client.send(websocket.TextMessage, []byte("已退出登录")); err != nil {
			zlog.Error(err.Error())
		}
		client.close()
	}
	return "退出成功", constants.BizCodeSuccess
}

// readLoop 读取 websocket 消息并发送给 Kafka 或 SendTo 通道
func (c *Client) readLoop() {
	zlog.Info("ws read goroutine start", zap.String("uuid", c.Uuid))
	defer c.close()
	for {
		_, jsonMessage, err := c.Conn.ReadMessage()
		if err != nil {
			zlog.Info("read message error, exiting readLoop", zap.Error(err), zap.String("uuid", c.Uuid))
			return
		}
		var message request.ChatMessageRequest
		if err := json.Unmarshal(jsonMessage, &message); err != nil {
			zlog.Error("json unmarshal error", zap.Error(err), zap.String("uuid", c.Uuid))
			continue
		}
		zlog.Info("received message", zap.String("uuid", c.Uuid), zap.ByteString("message", jsonMessage))

		// 向 Kafka 写入，并指定 Key 以保证分区一致性
		if err := myKafka.KafkaService.ChatWriter.WriteMessages(
			context.Background(),
			kafka.Message{Key: []byte(c.Uuid), Value: jsonMessage},
		); err != nil {
			zlog.Error("kafka write error", zap.Error(err), zap.String("uuid", c.Uuid))
		}
	}
}

// writeLoop 从 SendBack 通道读取消息并发送给 websocket
func (c *Client) writeLoop() {
	zlog.Info("ws write goroutine start", zap.String("uuid", c.Uuid))
	defer c.close()
	for messageBack := range c.SendBack {
		if err := c.send(websocket.TextMessage, messageBack.Message); err != nil {
			zlog.Info("write message error, exiting writeLoop", zap.Error(err), zap.String("uuid", c.Uuid))
			return
		}
		// 更新消息状态为已发送
		if res := dao.GormDB.Model(&model.Message{}).
			Where("uuid = ?", messageBack.Uuid).
			Update("status", message_status_enum.Sent); res.Error != nil {
			zlog.Error("db update error", zap.Error(res.Error), zap.String("uuid", c.Uuid))
		}
	}
}
