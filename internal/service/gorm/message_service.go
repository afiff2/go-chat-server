package gorm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/afiff2/go-chat-server/internal/config"
	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/dto/respond"
	"github.com/afiff2/go-chat-server/internal/model"
	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type messageService struct {
}

var MessageService = new(messageService)

// GetMessageList 获取聊天记录
func (m *messageService) GetMessageList(userOneId, userTwoId string) (string, []respond.GetMessageListRespond, int) {

	var messageList []model.Message
	if res := dao.GormDB.Where("(send_id = ? AND receive_id = ?) OR (send_id = ? AND receive_id = ?)", userOneId, userTwoId, userTwoId, userOneId).Order("created_at ASC").Find(&messageList); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	var rspList []respond.GetMessageListRespond
	for _, message := range messageList {
		rspList = append(rspList, respond.GetMessageListRespond{
			SendId:     message.SendId,
			SendName:   message.SendName,
			SendAvatar: message.SendAvatar,
			ReceiveId:  message.ReceiveId,
			Content:    message.Content,
			Url:        message.Url,
			Type:       message.Type,
			FileType:   message.FileType,
			FileName:   message.FileName,
			FileSize:   message.FileSize,
			CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return "获取聊天记录成功", rspList, constants.BizCodeSuccess
}

// GetGroupMessageList 获取群聊消息记录
func (m *messageService) GetGroupMessageList(groupId string) (string, []respond.GetGroupMessageListRespond, int) {
	cacheKey := "group_messagelist_" + groupId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("group_messagelist 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("group_messagelist 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
		var messageList []model.Message
		if res := dao.GormDB.Where("receive_id = ?", groupId).Order("created_at ASC").Find(&messageList); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}
		var rspList []respond.GetGroupMessageListRespond
		for _, message := range messageList {
			rsp := respond.GetGroupMessageListRespond{
				SendId:     message.SendId,
				SendName:   message.SendName,
				SendAvatar: message.SendAvatar,
				ReceiveId:  message.ReceiveId,
				Content:    message.Content,
				Url:        message.Url,
				Type:       message.Type,
				FileType:   message.FileType,
				FileName:   message.FileName,
				FileSize:   message.FileSize,
				CreatedAt:  message.CreatedAt.Format("2006-01-02 15:04:05"),
			}
			rspList = append(rspList, rsp)
		}
		if err := myredis.SetCache(cacheKey, &rspList); err != nil {
			zlog.Warn("预写 group_messagelist 缓存失败", zap.String("groupId", groupId), zap.Error(err))
		}
		return "获取聊天记录成功", rspList, constants.BizCodeSuccess
	}
	var rsp []respond.GetGroupMessageListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取聊天记录成功", rsp, constants.BizCodeSuccess
}

// UploadAvatar 上传头像
func (m *messageService) UploadAvatar(c *gin.Context) (string, int) {
	if err := c.Request.ParseMultipartForm(constants.FILE_MAX_SIZE); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	mForm := c.Request.MultipartForm
	for key, _ := range mForm.File {
		file, fileHeader, err := c.Request.FormFile(key)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		defer file.Close()
		zlog.Info(fmt.Sprintf("文件名：%s，文件大小：%d", fileHeader.Filename, fileHeader.Size))
		// 原来Filename应该是213451545.xxx，将Filename修改为avatar_ownerId.xxx
		ext := filepath.Ext(fileHeader.Filename)
		zlog.Info(ext)
		localFileName := config.GetConfig().StaticSrc.StaticAvatarPath + "/" + fileHeader.Filename
		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(localFileName), os.ModePerm); err != nil {
			zlog.Error("创建上传目录失败：", zap.Error(err))
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		out, err := os.Create(localFileName)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		zlog.Info("完成文件上传")
	}
	return "上传成功", constants.BizCodeSuccess
}

// UploadFile 上传文件
func (m *messageService) UploadFile(c *gin.Context) (string, int) {
	if err := c.Request.ParseMultipartForm(constants.FILE_MAX_SIZE); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	mForm := c.Request.MultipartForm
	for key, _ := range mForm.File {
		file, fileHeader, err := c.Request.FormFile(key)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		defer file.Close()
		zlog.Info(fmt.Sprintf("文件名：%s，文件大小：%d", fileHeader.Filename, fileHeader.Size))
		// 原来Filename应该是213451545.xxx，将Filename修改为avatar_ownerId.xxx
		ext := filepath.Ext(fileHeader.Filename)
		zlog.Info(ext)
		localFileName := config.GetConfig().StaticSrc.StaticFilePath + "/" + fileHeader.Filename
		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(localFileName), os.ModePerm); err != nil {
			zlog.Error("创建上传目录失败：", zap.Error(err))
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		out, err := os.Create(localFileName)
		if err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		zlog.Info("完成文件上传")
	}
	return "上传成功", constants.BizCodeSuccess
}
