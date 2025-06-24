package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/internal/dto/respond"
	"github.com/afiff2/go-chat-server/internal/model"
	"github.com/afiff2/go-chat-server/internal/service/kafka"
	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/enum/message/message_status_enum"
	"github.com/afiff2/go-chat-server/pkg/enum/message/message_type_enum"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type KafkaServer struct {
	Clients map[string]*Client
	mutex   sync.RWMutex //零值就是可用的锁，不需要显式赋值
}

var KafkaChatServer *KafkaServer

// 将https://127.0.0.1:8000/static/xxx 转为 /static/xxx
func normalizePath(path string) string {
	// 查找 "/static/" 的位置
	if path == "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png" {
		return path
	}
	staticIndex := strings.Index(path, "/static/")
	if staticIndex < 0 {
		zlog.Error("路径不合法", zap.String("path", path))
	}
	// 返回从 "/static/" 开始的部分
	return path[staticIndex:]
}

func init() {
	if KafkaChatServer == nil {
		KafkaChatServer = &KafkaServer{
			Clients: make(map[string]*Client),
		}
	}
	//signal.Notify(kafkaQuit, syscall.SIGINT, syscall.SIGTERM)
}

func (k *KafkaServer) Start(ctx context.Context) {
	// read chat message
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("kafka server panic", zap.Any("panic", r))
		}
	}()
	for {
		select {
		case <-ctx.Done():
			zlog.Info("KafkaServer.Start received shutdown signal, exiting")
			return
		default:
			kafkaMessage, err := kafka.KafkaService.ChatReader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					zlog.Info("Kafka read cancelled, exiting loop")
					return
				}
				zlog.Error("kafka read error", zap.Error(err))
				time.Sleep(100 * time.Millisecond) //防止busy loop，Kafka 短暂不可用、不断抛错
				continue
			}
			zlog.Info(fmt.Sprintf("topic=%s, partition=%d, offset=%d, key=%s, value=%s", kafkaMessage.Topic, kafkaMessage.Partition, kafkaMessage.Offset, kafkaMessage.Key, kafkaMessage.Value))
			data := kafkaMessage.Value
			var chatMessageReq request.ChatMessageRequest
			if err := json.Unmarshal(data, &chatMessageReq); err != nil {
				zlog.Error(err.Error())
			}
			zlog.Info(fmt.Sprintf("原消息为：%v, 反序列化后为：%v", data, chatMessageReq))
			switch chatMessageReq.Type {
			case message_type_enum.Text:
				// 存message
				message := model.Message{
					Uuid:       "M" + uuid.NewString(),
					SessionId:  chatMessageReq.SessionId,
					Type:       chatMessageReq.Type,
					Content:    chatMessageReq.Content,
					Url:        "",
					SendId:     chatMessageReq.SendId,
					SendName:   chatMessageReq.SendName,
					SendAvatar: chatMessageReq.SendAvatar,
					ReceiveId:  chatMessageReq.ReceiveId,
					FileSize:   "0B",
					FileType:   "",
					FileName:   "",
					Status:     message_status_enum.Unsent,
					CreatedAt:  time.Now(),
					AVdata:     "",
				}
				// 对SendAvatar去除前面/static之前的所有内容，防止ip前缀引入
				message.SendAvatar = normalizePath(message.SendAvatar)
				if res := dao.GormDB.Create(&message); res.Error != nil {
					zlog.Error(res.Error.Error())
				}
				switch message.ReceiveId[0] {
				case 'U':
					// 如果能找到ReceiveId，说明在线，可以发送，否则存表后跳过
					messageRsp := respond.GetMessageListRespond{
						SendId:     message.SendId,
						SendName:   message.SendName,
						SendAvatar: chatMessageReq.SendAvatar,
						ReceiveId:  message.ReceiveId,
						Type:       message.Type,
						Content:    message.Content,
						Url:        message.Url,
						FileSize:   message.FileSize,
						FileName:   message.FileName,
						FileType:   message.FileType,
						CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
					}
					jsonMessage, err := json.Marshal(messageRsp)
					if err != nil {
						zlog.Error(err.Error())
					}
					zlog.Info(fmt.Sprintf("返回的消息为：%v, 序列化后为：%v", messageRsp, jsonMessage))
					var messageBack = &MessageBack{
						Message: jsonMessage,
						Uuid:    message.Uuid,
					}
					k.mutex.Lock()
					if receiveClient, ok := k.Clients[message.ReceiveId]; ok {
						receiveClient.SendBack <- messageBack // 向client.Send发送
					}
					// 回显,确保存表
					sendClient := k.Clients[message.SendId]
					sendClient.SendBack <- messageBack
					k.mutex.Unlock()
				case 'G':
					messageRsp := respond.GetGroupMessageListRespond{
						SendId:     message.SendId,
						SendName:   message.SendName,
						SendAvatar: chatMessageReq.SendAvatar,
						ReceiveId:  message.ReceiveId,
						Type:       message.Type,
						Content:    message.Content,
						Url:        message.Url,
						FileSize:   message.FileSize,
						FileName:   message.FileName,
						FileType:   message.FileType,
						CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
					}
					jsonMessage, err := json.Marshal(messageRsp)
					if err != nil {
						zlog.Error(err.Error())
					}
					zlog.Info(fmt.Sprintf("返回的消息为：%v, 序列化后为：%v", messageRsp, jsonMessage))
					var messageBack = &MessageBack{
						Message: jsonMessage,
						Uuid:    message.Uuid,
					}

					var members []model.GroupMember
					if res := dao.GormDB.
						Where("group_uuid = ?", message.ReceiveId).
						Find(&members); res.Error != nil {
						zlog.Error(res.Error.Error())
					}

					k.mutex.Lock()
					for _, member := range members {
						if member.UserUuid != message.SendId {
							if receiveClient, ok := k.Clients[member.UserUuid]; ok {
								receiveClient.SendBack <- messageBack
							}
						} else {
							sendClient := k.Clients[message.SendId]
							sendClient.SendBack <- messageBack
						}
					}
					k.mutex.Unlock()
					// redis （写回可能不同步）
					if err := myredis.DelKeyIfExists("group_messagelist_" + message.ReceiveId); err != nil {
						zlog.Error(err.Error())
					}
				}
			case message_type_enum.File:
				// 存message
				message := model.Message{
					Uuid:       "M" + uuid.NewString(),
					SessionId:  chatMessageReq.SessionId,
					Type:       chatMessageReq.Type,
					Content:    "",
					Url:        chatMessageReq.Url,
					SendId:     chatMessageReq.SendId,
					SendName:   chatMessageReq.SendName,
					SendAvatar: chatMessageReq.SendAvatar,
					ReceiveId:  chatMessageReq.ReceiveId,
					FileSize:   chatMessageReq.FileSize,
					FileType:   chatMessageReq.FileType,
					FileName:   chatMessageReq.FileName,
					Status:     message_status_enum.Unsent,
					CreatedAt:  time.Now(),
					AVdata:     "",
				}
				// 对SendAvatar去除前面/static之前的所有内容，防止ip前缀引入
				message.SendAvatar = normalizePath(message.SendAvatar)
				if res := dao.GormDB.Create(&message); res.Error != nil {
					zlog.Error(res.Error.Error())
				}
				switch message.ReceiveId[0] {
				case 'U':
					messageRsp := respond.GetMessageListRespond{
						SendId:     message.SendId,
						SendName:   message.SendName,
						SendAvatar: chatMessageReq.SendAvatar,
						ReceiveId:  message.ReceiveId,
						Type:       message.Type,
						Content:    message.Content,
						Url:        message.Url,
						FileSize:   message.FileSize,
						FileName:   message.FileName,
						FileType:   message.FileType,
						CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
					}
					jsonMessage, err := json.Marshal(messageRsp)
					if err != nil {
						zlog.Error(err.Error())
					}
					zlog.Info(fmt.Sprintf("返回的消息为：%v, 序列化后为：%v", messageRsp, jsonMessage))
					var messageBack = &MessageBack{
						Message: jsonMessage,
						Uuid:    message.Uuid,
					}
					k.mutex.Lock()
					if receiveClient, ok := k.Clients[message.ReceiveId]; ok {
						//messageBack.Message = jsonMessage
						//messageBack.Uuid = message.Uuid
						receiveClient.SendBack <- messageBack // 向client.Send发送
					}
					// 因为send_id肯定在线，所以这里在后端进行在线回显message，其实优化的话前端可以直接回显
					// 问题在于前后端的req和rsp结构不同，前端存储message的messageList不能存req，只能存rsp
					// 所以这里后端进行回显，前端不回显
					sendClient := k.Clients[message.SendId]
					sendClient.SendBack <- messageBack
					k.mutex.Unlock()
				case 'G':
					messageRsp := respond.GetGroupMessageListRespond{
						SendId:     message.SendId,
						SendName:   message.SendName,
						SendAvatar: chatMessageReq.SendAvatar,
						ReceiveId:  message.ReceiveId,
						Type:       message.Type,
						Content:    message.Content,
						Url:        message.Url,
						FileSize:   message.FileSize,
						FileName:   message.FileName,
						FileType:   message.FileType,
						CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
					}
					jsonMessage, err := json.Marshal(messageRsp)
					if err != nil {
						zlog.Error(err.Error())
					}
					zlog.Info(fmt.Sprintf("返回的消息为：%v, 序列化后为：%v", messageRsp, jsonMessage))
					var messageBack = &MessageBack{
						Message: jsonMessage,
						Uuid:    message.Uuid,
					}

					var members []model.GroupMember
					if res := dao.GormDB.
						Where("group_uuid = ?", message.ReceiveId).
						Find(&members); res.Error != nil {
						zlog.Error(res.Error.Error())
					}

					k.mutex.Lock()
					for _, member := range members {
						if member.UserUuid != message.SendId {
							if receiveClient, ok := k.Clients[member.UserUuid]; ok {
								receiveClient.SendBack <- messageBack
							}
						} else {
							sendClient := k.Clients[message.SendId]
							sendClient.SendBack <- messageBack
						}
					}
					k.mutex.Unlock()

					// redis （写回可能不同步）
					if err := myredis.DelKeyIfExists("group_messagelist_" + message.ReceiveId); err != nil {
						zlog.Error(err.Error())
					}
				}
			case message_type_enum.AudioOrVideo:
				var avData request.AVData
				if err := json.Unmarshal([]byte(chatMessageReq.AVdata), &avData); err != nil {
					zlog.Error(err.Error())
				}
				message := model.Message{
					Uuid:       "M" + uuid.NewString(),
					SessionId:  chatMessageReq.SessionId,
					Type:       chatMessageReq.Type,
					Content:    "",
					Url:        "",
					SendId:     chatMessageReq.SendId,
					SendName:   chatMessageReq.SendName,
					SendAvatar: chatMessageReq.SendAvatar,
					ReceiveId:  chatMessageReq.ReceiveId,
					FileSize:   "",
					FileType:   "",
					FileName:   "",
					Status:     message_status_enum.Unsent,
					CreatedAt:  time.Now(),
					AVdata:     chatMessageReq.AVdata,
				}
				if avData.MessageId == "PROXY" && (avData.Type == "start_call" || avData.Type == "receive_call" || avData.Type == "reject_call") {
					// 存message
					// 对SendAvatar去除前面/static之前的所有内容，防止ip前缀引入
					message.SendAvatar = normalizePath(message.SendAvatar)
					if res := dao.GormDB.Create(&message); res.Error != nil {
						zlog.Error(res.Error.Error())
					}
				}

				if chatMessageReq.ReceiveId[0] == 'U' { // 发送给User
					// 如果能找到ReceiveId，说明在线，可以发送，否则存表后跳过
					// 因为在线的时候是通过websocket更新消息记录的，离线后通过存表，登录时只调用一次数据库操作
					// 切换chat对象后，前端的messageList也会改变，获取messageList从第二次就是从redis中获取
					messageRsp := respond.AVMessageRespond{
						SendId:     message.SendId,
						SendName:   message.SendName,
						SendAvatar: message.SendAvatar,
						ReceiveId:  message.ReceiveId,
						Type:       message.Type,
						Content:    message.Content,
						Url:        message.Url,
						FileSize:   message.FileSize,
						FileName:   message.FileName,
						FileType:   message.FileType,
						CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
						AVdata:     message.AVdata,
					}
					jsonMessage, err := json.Marshal(messageRsp)
					if err != nil {
						zlog.Error(err.Error())
					}
					zlog.Info(fmt.Sprintf("返回的消息为：%v, 序列化后为：%v", messageRsp, jsonMessage))
					var messageBack = &MessageBack{
						Message: jsonMessage,
						Uuid:    message.Uuid,
					}
					k.mutex.Lock()
					if receiveClient, ok := k.Clients[message.ReceiveId]; ok {
						//messageBack.Message = jsonMessage
						//messageBack.Uuid = message.Uuid
						receiveClient.SendBack <- messageBack // 向client.Send发送
					}
					// 通话这不能回显，发回去的话就会出现两个start_call。
					k.mutex.Unlock()
				}
			}
		}
	}

}

// GetClient 返回指定 uuid 的 client，以及是否存在
func (k *KafkaServer) GetClient(uuid string) (*Client, bool) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	c, ok := k.Clients[uuid]
	return c, ok
}

func (k *KafkaServer) AddClient(client *Client) {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	k.Clients[client.Uuid] = client
}

func (k *KafkaServer) RemoveClient(uuid string) {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	delete(k.Clients, uuid)
}
