package gorm

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/internal/dto/respond"
	"github.com/afiff2/go-chat-server/internal/model"
	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/enum/contact/contact_status_enum"
	"github.com/afiff2/go-chat-server/pkg/enum/group_info/group_status_enum"
	"github.com/afiff2/go-chat-server/pkg/enum/user_info/user_status_enum"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type sessionService struct {
}

var SessionService = new(sessionService)

// CreateSession 创建会话
func (s *sessionService) CreateSession(req request.OpenSessionRequest) (string, string, int) {
	if len(req.ReceiveId) == 0 {
		return "目标ID不能为空", "", constants.BizCodeInvalid
	}
	if req.SendId == req.ReceiveId {
		return "不能自己和自己建立会话", "", constants.BizCodeInvalid
	}
	var user model.UserInfo
	if res := dao.GormDB.Where("uuid = ?", req.SendId).First(&user); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, "", constants.BizCodeError
	}
	// 校验 SendId 状态
	if user.Status == user_status_enum.DISABLE {
		return "发送用户被禁用", "", constants.BizCodeInvalid
	}
	var session model.Session
	session.Uuid = "S" + uuid.NewString()
	session.SendId = req.SendId
	session.ReceiveId = req.ReceiveId
	session.CreatedAt = time.Now()
	if req.ReceiveId[0] == 'U' {
		var receiveUser model.UserInfo
		if res := dao.GormDB.Where("uuid = ?", req.ReceiveId).First(&receiveUser); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, "", constants.BizCodeError
		}
		if receiveUser.Status == user_status_enum.DISABLE {
			zlog.Error("该用户被禁用了")
			return "该用户被禁用了", "", constants.BizCodeInvalid
		} else {
			session.ReceiveName = receiveUser.Nickname
			session.Avatar = receiveUser.Avatar
		}
	} else {
		var receiveGroup model.GroupInfo
		if res := dao.GormDB.Where("uuid = ?", req.ReceiveId).First(&receiveGroup); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, "", constants.BizCodeError
		}
		if receiveGroup.Status == group_status_enum.DISABLE {
			zlog.Error("该群聊被禁用了")
			return "该群聊被禁用了", "", constants.BizCodeInvalid
		} else {
			session.ReceiveName = receiveGroup.Name
			session.Avatar = receiveGroup.Avatar
		}
	}

	if res := dao.GormDB.Create(&session); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, "", constants.BizCodeError
	}
	if err := SetCache("session_"+req.SendId+"_"+req.ReceiveId, &session); err != nil {
		zlog.Warn("预写 session 缓存失败", zap.String("SendId", req.SendId), zap.String("ReceiveId", req.ReceiveId), zap.Error(err))
	}
	if req.ReceiveId[0] == 'G' {
		if err := myredis.DelKeysWithPattern("group_session_list_" + req.SendId); err != nil {
			zlog.Error(err.Error())
		}
	}
	if req.ReceiveId[0] == 'U' {
		if err := myredis.DelKeysWithPattern("session_list_" + req.SendId); err != nil {
			zlog.Error(err.Error())
		}
	}
	return "会话创建成功", session.Uuid, constants.BizCodeSuccess
}

// CheckOpenSessionAllowed 检查是否允许发起会话
func (s *sessionService) CheckOpenSessionAllowed(sendId, receiveId string) (string, bool, int) {
	var contact model.UserContact
	if res := dao.GormDB.Where("user_id = ? and contact_id = ?", sendId, receiveId).First(&contact); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return "未添加联系人，无效申请", false, constants.BizCodeInvalid
		} else {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, false, constants.BizCodeError
		}
	}
	if contact.Status == contact_status_enum.BE_BLACK {
		return "已被对方拉黑，无法发起会话", false, constants.BizCodeInvalid
	} else if contact.Status == contact_status_enum.BLACK {
		return "已拉黑对方，先解除拉黑状态才能发起会话", false, constants.BizCodeInvalid
	}
	if receiveId[0] == 'U' {
		var user model.UserInfo
		if res := dao.GormDB.Where("uuid = ?", receiveId).First(&user); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, false, constants.BizCodeError
		}
		if user.Status == user_status_enum.DISABLE {
			zlog.Info("对方已被禁用，无法发起会话")
			return "对方已被禁用，无法发起会话", false, constants.BizCodeInvalid
		}
	} else {
		var group model.GroupInfo
		if res := dao.GormDB.Where("uuid = ?", receiveId).First(&group); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, false, constants.BizCodeError
		}
		if group.Status == group_status_enum.DISABLE {
			zlog.Info("对方已被禁用，无法发起会话")
			return "对方已被禁用，无法发起会话", false, constants.BizCodeInvalid
		}
	}
	return "可以发起会话", true, constants.BizCodeSuccess
}

// OpenSession 打开会话
func (s *sessionService) OpenSession(req request.OpenSessionRequest) (string, string, int) {
	cacheKey := "session_" + req.SendId + "_" + req.ReceiveId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("session 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("session 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
		var session model.Session
		if res := dao.GormDB.Where("send_id = ? and receive_id = ?", req.SendId, req.ReceiveId).First(&session); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				zlog.Info("会话没有找到，将新建会话")
				createReq := request.OpenSessionRequest{
					SendId:    req.SendId,
					ReceiveId: req.ReceiveId,
				}
				return s.CreateSession(createReq)
			}
		}
		if err := SetCache("session_"+req.SendId+"_"+req.ReceiveId, &session); err != nil {
			zlog.Warn("预写 session 缓存失败", zap.String("SendId", req.SendId), zap.String("ReceiveId", req.ReceiveId), zap.Error(err))
		}
		return "会话创建成功", session.Uuid, constants.BizCodeSuccess
	}
	var session model.Session
	if err := json.Unmarshal([]byte(rspString), &session); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, "", constants.BizCodeError
	}
	return "会话创建成功", session.Uuid, constants.BizCodeSuccess
}

// GetUserSessionList 获取用户会话列表
func (s *sessionService) GetUserSessionList(ownerId string) (string, []respond.UserSessionListRespond, int) {
	cacheKey := "session_list_" + ownerId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("session_list 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("session_list 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
		var sessionList []model.Session
		if res := dao.GormDB.Order("created_at DESC").Where("send_id = ?", ownerId).Find(&sessionList); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}
		var sessionListRsp []respond.UserSessionListRespond
		for i := 0; i < len(sessionList); i++ {
			if sessionList[i].ReceiveId[0] == 'U' {
				sessionListRsp = append(sessionListRsp, respond.UserSessionListRespond{
					SessionId: sessionList[i].Uuid,
					Avatar:    sessionList[i].Avatar,
					UserId:    sessionList[i].ReceiveId,
					Username:  sessionList[i].ReceiveName,
				})
			}
		}
		if len(sessionListRsp) == 0 {
			return "未创建用户会话", nil, constants.BizCodeSuccess
		}
		if err := SetCache("session_list_"+ownerId, &sessionListRsp); err != nil {
			zlog.Warn("预写 session_list 缓存失败", zap.String("ownerId", ownerId), zap.Error(err))
		}
		return "获取成功", sessionListRsp, constants.BizCodeSuccess
	}
	var rsp []respond.UserSessionListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	return "获取成功", rsp, constants.BizCodeSuccess
}

// GetGroupSessionList 获取群聊会话列表
func (s *sessionService) GetGroupSessionList(ownerId string) (string, []respond.GroupSessionListRespond, int) {
	cacheKey := "group_session_list_" + ownerId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("group_session_list 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("group_session_list 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
		var sessionList []model.Session
		if res := dao.GormDB.Order("created_at DESC").Where("send_id = ?", ownerId).Find(&sessionList); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}
		var sessionListRsp []respond.GroupSessionListRespond
		for i := 0; i < len(sessionList); i++ {
			if sessionList[i].ReceiveId[0] == 'G' {
				sessionListRsp = append(sessionListRsp, respond.GroupSessionListRespond{
					SessionId: sessionList[i].Uuid,
					Avatar:    sessionList[i].Avatar,
					GroupId:   sessionList[i].ReceiveId,
					GroupName: sessionList[i].ReceiveName,
				})
			}
		}
		if len(sessionListRsp) == 0 {
			return "未创建群聊会话", nil, constants.BizCodeSuccess
		}
		if err := SetCache("group_session_list_"+ownerId, &sessionListRsp); err != nil {
			zlog.Warn("预写 group_session_list 缓存失败", zap.String("ownerId", ownerId), zap.Error(err))
		}
		return "获取成功", sessionListRsp, constants.BizCodeSuccess
	}
	var rsp []respond.GroupSessionListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	return "获取成功", rsp, constants.BizCodeSuccess
}

// DeleteSession 删除会话
func (s *sessionService) DeleteSession(ownerId, ReceiveId, sessionId string) (string, int) {

	res := dao.GormDB.Where("uuid = ? AND send_id = ?", sessionId, ownerId).Delete(&model.Session{})
	if res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if res.RowsAffected == 0 {
		return "会话不存在或无权限", constants.BizCodeInvalid
	}
	if err := myredis.DelKeyIfExists("session_" + ownerId + "_" + ReceiveId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeyIfExists("group_session_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeyIfExists("session_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}
	return "删除成功", constants.BizCodeSuccess
}
