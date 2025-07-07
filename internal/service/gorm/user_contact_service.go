package gorm

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/internal/dto/respond"
	"github.com/afiff2/go-chat-server/internal/model"
	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/enum/contact/contact_status_enum"
	"github.com/afiff2/go-chat-server/pkg/enum/contact/contact_type_enum"
	"github.com/afiff2/go-chat-server/pkg/enum/contact_apply/contact_apply_status_enum"
	"github.com/afiff2/go-chat-server/pkg/enum/group_info/group_status_enum"
	"github.com/afiff2/go-chat-server/pkg/enum/user_info/user_status_enum"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userContactService struct {
}

var UserContactService = new(userContactService)

// GetUserList 获取用户列表
// 关于用户被禁用的问题，这里查到的是所有联系人，如果被禁用或被拉黑会以弹窗的形式提醒，无法打开会话框；如果被删除，是搜索不到该联系人的。
func (u *userContactService) GetUserList(ownerId string) (string, []respond.MyUserListRespond, int) {
	cacheKey := "contact_user_list_" + ownerId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("contact_user_list 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("contact_user_list 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}

		var userList []respond.MyUserListRespond
		err := dao.GormDB.
			Model(&model.UserContact{}).
			Select("u.uuid   AS user_id, "+
				"u.nickname AS user_name, "+
				"u.avatar   AS avatar").
			Joins("INNER JOIN user_info AS u ON u.uuid = user_contact.contact_id").
			Where("user_contact.user_id = ? AND user_contact.contact_type = ?", ownerId, contact_type_enum.USER).
			Order("user_contact.created_at DESC").
			Scan(&userList).Error
		//Scan没有查到不会返回错误
		if err != nil {
			zlog.Error("查询用户列表失败", zap.Error(err))
			return constants.SYSTEM_ERROR, nil, constants.BizCodeInvalid
		}

		if err := myredis.SetCache("contact_user_list_"+ownerId, &userList); err != nil {
			zlog.Warn("预写 contact_user_list 缓存失败", zap.String("ownerId", ownerId), zap.Error(err))
		}
		return "获取用户列表成功", userList, constants.BizCodeSuccess
	}
	var rsp []respond.MyUserListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	return "获取用户列表成功", rsp, constants.BizCodeSuccess
}

// LoadMyJoinedGroup 获取我加入的群聊
func (u *userContactService) LoadMyJoinedGroup(ownerId string) (string, []respond.LoadMyJoinedGroupRespond, int) {
	cacheKey := "my_joined_group_list_" + ownerId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("my_joined_group_list 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("my_joined_group_list 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}

		var groupList []respond.LoadMyJoinedGroupRespond
		err := dao.GormDB.
			Model(&model.UserContact{}).
			Select("g.uuid AS group_id, g.name AS group_name, g.avatar").
			Joins("JOIN group_info AS g ON user_contact.contact_id = g.uuid").
			Where("user_contact.user_id = ? AND g.status = ?", ownerId, group_status_enum.NORMAL).
			Order("user_contact.created_at DESC").
			Scan(&groupList).Error
		if err != nil {
			zlog.Error("查询加入的群聊失败", zap.Error(err))
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}

		if err := myredis.SetCache("my_joined_group_list_"+ownerId, &groupList); err != nil {
			zlog.Warn("预写 my_joined_group_list 缓存失败", zap.String("ownerId", ownerId), zap.Error(err))
		}
		return "获取加入群成功", groupList, constants.BizCodeSuccess
	}
	var rsp []respond.LoadMyJoinedGroupRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	return "获取加入群成功", rsp, constants.BizCodeSuccess
}

// GetContactInfo 获取联系人信息
// 调用这个接口的前提是该联系人没有处在删除或被删除，或者该用户还在群聊中
func (u *userContactService) GetContactInfo(contactId string) (string, respond.GetContactInfoRespond, int) {
	//防止contactId[0]崩溃
	if len(contactId) == 0 {
		zlog.Warn("contactId 为空")
		return "无效的联系人ID", respond.GetContactInfoRespond{}, constants.BizCodeInvalid
	}

	cacheKey := "contact_info_" + contactId
	cached, err := myredis.GetKeyNilIsErr(cacheKey)
	if err == nil {
		var cachedResp respond.GetContactInfoRespond
		if err := json.Unmarshal([]byte(cached), &cachedResp); err == nil {
			zlog.Debug("联系人信息命中缓存", zap.String("contactId", contactId))
			return "获取联系人信息成功", cachedResp, constants.BizCodeSuccess
		} else {
			zlog.Warn("联系人信息缓存解析失败，尝试回源", zap.Error(err))
		}
	} else {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("contact_info_ 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("contact_info_ 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
	}

	if contactId[0] == 'G' {
		var group model.GroupInfo
		if res := dao.GormDB.First(&group, "uuid = ?", contactId); res.Error != nil {
			zlog.Error("查询群聊信息失败", zap.String("groupId", contactId), zap.Error(res.Error))
			return constants.SYSTEM_ERROR, respond.GetContactInfoRespond{}, constants.BizCodeError
		}
		if group.Status == group_status_enum.DISABLE {
			zlog.Info("该群聊处于禁用状态", zap.String("groupId", contactId))
			return "该群聊处于禁用状态", respond.GetContactInfoRespond{}, constants.BizCodeInvalid
		}

		resp := respond.GetContactInfoRespond{
			ContactId:        group.Uuid,
			ContactName:      group.Name,
			ContactAvatar:    group.Avatar,
			ContactNotice:    group.Notice,
			ContactAddMode:   group.AddMode,
			ContactMemberCnt: group.MemberCnt,
			ContactOwnerId:   group.OwnerId,
		}
		if err := myredis.SetCache("contact_info_"+contactId, &resp); err != nil {
			zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", contactId), zap.Error(err))
		}
		return "获取联系人信息成功", resp, constants.BizCodeSuccess
	}

	if contactId[0] == 'U' {
		var user model.UserInfo
		if res := dao.GormDB.First(&user, "uuid = ?", contactId); res.Error != nil {
			zlog.Error("查询用户信息失败", zap.String("userId", contactId), zap.Error(res.Error))
			return constants.SYSTEM_ERROR, respond.GetContactInfoRespond{}, constants.BizCodeError
		}

		rsp := respond.GetUserInfoRespond{
			Uuid:      user.Uuid,
			Telephone: user.Telephone,
			Nickname:  user.Nickname,
			Avatar:    user.Avatar,
			Birthday:  user.Birthday,
			Email:     user.Email,
			Gender:    user.Gender,
			Signature: user.Signature,
			CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
			IsAdmin:   user.IsAdmin,
			Status:    user.Status,
		}
		// 将用户信息写入 Redis 缓存
		if err := myredis.SetCache("user_info_"+rsp.Uuid, &rsp); err != nil {
			zlog.Warn("写入 Redis 缓存失败", zap.Error(err))
		}

		if user.Status == user_status_enum.DISABLE {
			zlog.Info("该用户处于禁用状态", zap.String("userId", contactId))
			return "该用户处于禁用状态", respond.GetContactInfoRespond{}, constants.BizCodeInvalid
		}

		resp := respond.GetContactInfoRespond{
			ContactId:        user.Uuid,
			ContactName:      user.Nickname,
			ContactAvatar:    user.Avatar,
			ContactBirthday:  user.Birthday,
			ContactEmail:     user.Email,
			ContactPhone:     user.Telephone,
			ContactGender:    user.Gender,
			ContactSignature: user.Signature,
		}
		if err := myredis.SetCache("contact_info_"+contactId, &resp); err != nil {
			zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", contactId), zap.Error(err))
		}
		return "获取联系人信息成功", resp, constants.BizCodeSuccess
	}
	return constants.SYSTEM_ERROR, respond.GetContactInfoRespond{}, constants.BizCodeError
}

// DeleteContact 删除联系人（只包含用户）
func (u *userContactService) DeleteContact(ownerId, contactId string) (string, int) {
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除 owner -> contact 关系
	if err := tx.Where("user_id = ? AND contact_id = ?", ownerId, contactId).
		Delete(&model.UserContact{}).Error; err != nil {
		tx.Rollback()
		zlog.Error("软删除联系人失败 owner -> contact", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除 contact -> owner 关系
	if err := tx.Where("user_id = ? AND contact_id = ?", contactId, ownerId).
		Delete(&model.UserContact{}).Error; err != nil {
		tx.Rollback()
		zlog.Error("软删除联系人失败 contact -> owner", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除会话记录（双方）
	if err := tx.Where("send_id = ? AND receive_id = ?", ownerId, contactId).
		Delete(&model.Session{}).Error; err != nil {
		tx.Rollback()
		zlog.Error("软删除会话失败 A->B", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := tx.Where("send_id = ? AND receive_id = ?", contactId, ownerId).
		Delete(&model.Session{}).Error; err != nil {
		tx.Rollback()
		zlog.Error("软删除会话失败 B->A", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除申请记录
	if err := tx.Where("contact_id = ? AND user_id = ?", ownerId, contactId).
		Delete(&model.ContactApply{}).Error; err != nil {
		tx.Rollback()
		zlog.Error("软删除申请记录失败 A->B", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := tx.Where("contact_id = ? AND user_id = ?", contactId, ownerId).
		Delete(&model.ContactApply{}).Error; err != nil {
		tx.Rollback()
		zlog.Error("软删除申请记录失败 B->A", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除缓存
	if err := myredis.DelKeyIfExists("contact_user_list_" + ownerId); err != nil {
		zlog.Error("删除联系人缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeyIfExists("contact_user_list_" + contactId); err != nil {
		zlog.Error("删除联系人缓存失败", zap.Error(err))
	}

	return "删除联系人成功", constants.BizCodeSuccess
}

// ApplyContact 申请添加联系人
func (u *userContactService) ApplyContact(req request.ApplyContactRequest) (string, int) {
	if len(req.ContactId) == 0 {
		return "非法联系人 ID", constants.BizCodeInvalid
	}
	if req.OwnerId == req.ContactId {
		return "不能添加自己为联系人", constants.BizCodeInvalid
	}

	var contactType int8
	var contactExists bool

	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 判断是用户还是群聊，并校验存在性和状态
	switch req.ContactId[0] {
	case 'U':
		var user model.UserInfo
		if err := tx.First(&user, "uuid = ?", req.ContactId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return "该用户不存在", constants.BizCodeInvalid
			}
			zlog.Error("查询用户失败", zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		if user.Status == user_status_enum.DISABLE {
			tx.Rollback()
			return "用户已被禁用", constants.BizCodeInvalid
		}
		contactExists = true
		contactType = contact_type_enum.USER

	case 'G':
		var group model.GroupInfo
		if err := tx.First(&group, "uuid = ?", req.ContactId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return "该群聊不存在", constants.BizCodeInvalid
			}
			zlog.Error("查询群聊失败", zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		if group.Status == group_status_enum.DISABLE {
			tx.Rollback()
			return "群聊已被禁用", constants.BizCodeInvalid
		}
		contactExists = true
		contactType = contact_type_enum.GROUP

	default:
		tx.Rollback()
		return "非法联系人类型", constants.BizCodeInvalid
	}

	if !contactExists {
		tx.Rollback()
		return "联系人不存在", constants.BizCodeInvalid
	}

	// 查询现有的申请记录
	var contactApply model.ContactApply
	err := tx.Where("user_id = ? AND contact_id = ?", req.OwnerId, req.ContactId).First(&contactApply).Error

	//没有历史申请
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新申请记录
		contactApply = model.ContactApply{
			Uuid:        "A" + uuid.NewString(),
			UserId:      req.OwnerId,
			ContactId:   req.ContactId,
			ContactType: contactType,
			Status:      contact_apply_status_enum.PENDING,
			Message:     req.Message,
			LastApplyAt: time.Now(),
		}
		if err := tx.Create(&contactApply).Error; err != nil {
			zlog.Error("创建申请记录失败", zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		if err := tx.Commit().Error; err != nil {
			zlog.Error("事务提交失败", zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		return "申请成功", constants.BizCodeSuccess

	} else if err != nil {
		zlog.Error("查询申请记录失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 存在申请记录
	if contactApply.Status == contact_apply_status_enum.BLACK {
		tx.Rollback()
		return "对方已将你拉黑", constants.BizCodeInvalid
	}

	// 更新申请时间和状态
	contactApply.LastApplyAt = time.Now()
	contactApply.Status = contact_apply_status_enum.PENDING
	contactApply.Message = req.Message // 可选：更新申请备注

	if err := tx.Save(&contactApply).Error; err != nil {
		zlog.Error("更新申请记录失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	return "申请成功", constants.BizCodeSuccess
}

// GetNewContactList 获取新的联系人申请列表
func (u *userContactService) GetNewContactList(ownerId string) (string, []respond.NewContactListRespond, int) {
	var applyList []model.ContactApply
	if err := dao.GormDB.
		Where("contact_id = ? AND status = ?", ownerId, contact_apply_status_enum.PENDING).
		Order("last_apply_at DESC").
		Find(&applyList).Error; err != nil {
		zlog.Error("获取联系人申请失败", zap.Error(err))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	if len(applyList) == 0 {
		return "没有在申请的联系人", nil, constants.BizCodeSuccess
	}

	// 批量查用户信息
	userIds := make([]string, 0, len(applyList))
	for _, apply := range applyList {
		userIds = append(userIds, apply.UserId)
	}
	var users []model.UserInfo
	if err := dao.GormDB.Where("uuid IN ?", userIds).Find(&users).Error; err != nil {
		zlog.Error("批量查询用户信息失败", zap.Error(err))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	userMap := make(map[string]model.UserInfo)
	for _, u := range users {
		userMap[u.Uuid] = u
	}

	// 组装响应
	var rsp []respond.NewContactListRespond
	for _, apply := range applyList {
		user := userMap[apply.UserId]
		var message string
		if apply.Message == "" {
			message = "申请理由：无"
		} else {
			message = "申请理由：" + strings.TrimSpace(apply.Message)
		}
		rsp = append(rsp, respond.NewContactListRespond{
			ContactId:     user.Uuid,
			ContactName:   user.Nickname,
			ContactAvatar: user.Avatar,
			Message:       message,
		})
	}
	return "获取成功", rsp, constants.BizCodeSuccess
}

// GetAddGroupList 获取新的加群列表
// 前端已经判断调用接口的用户是群主，也只有群主才能调用这个接口
func (u *userContactService) GetAddGroupList(groupId string) (string, []respond.AddGroupListRespond, int) {
	var applyList []model.ContactApply
	err := dao.GormDB.
		Where("contact_id = ? AND status = ?", groupId, contact_apply_status_enum.PENDING).
		Order("last_apply_at DESC").
		Find(&applyList).Error
	if err != nil {
		zlog.Error("查询加群申请失败", zap.Error(err))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	if len(applyList) == 0 {
		return "没有在申请的联系人", nil, constants.BizCodeSuccess
	}

	// 批量查用户信息
	userIds := make([]string, 0, len(applyList))
	for _, apply := range applyList {
		userIds = append(userIds, apply.UserId)
	}

	var users []model.UserInfo
	if err := dao.GormDB.Where("uuid IN ?", userIds).Find(&users).Error; err != nil {
		zlog.Error("批量查询用户信息失败", zap.Error(err))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	userMap := make(map[string]model.UserInfo)
	for _, u := range users {
		userMap[u.Uuid] = u
	}

	// 构建响应
	var rsp []respond.AddGroupListRespond
	for _, apply := range applyList {
		user := userMap[apply.UserId]
		var message string
		if apply.Message == "" {
			message = "申请理由：无"
		} else {
			message = "申请理由：" + strings.TrimSpace(apply.Message)
		}
		rsp = append(rsp, respond.AddGroupListRespond{
			ContactId:     user.Uuid,
			ContactName:   user.Nickname,
			ContactAvatar: user.Avatar,
			Message:       message,
		})
	}

	return "获取成功", rsp, constants.BizCodeSuccess
}

// PassContactApply 通过联系人申请
// ownerId 如果是用户的话就是登录用户，如果是群聊的话就是群聊id
func (u *userContactService) PassContactApply(ownerId string, contactId string) (string, int) {
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("PassContactApply panic", zap.Any("recover", r))
			tx.Rollback()
		}
	}()

	var contactApply model.ContactApply
	if err := tx.Where("contact_id = ? AND user_id = ?", ownerId, contactId).First(&contactApply).Error; err != nil {
		zlog.Error("查询申请记录失败", zap.String("ownerId", ownerId), zap.String("contactId", contactId), zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 用户类型处理
	if ownerId[0] == 'U' {
		var user model.UserInfo
		if err := tx.First(&user, "uuid = ?", contactId).Error; err != nil {
			zlog.Error("查询用户失败", zap.String("userId", contactId), zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		if user.Status == user_status_enum.DISABLE {
			zlog.Info("用户已被禁用", zap.String("userId", contactId))
			tx.Rollback()
			return "用户已被禁用", constants.BizCodeInvalid
		}

		// 更新申请状态
		contactApply.Status = contact_apply_status_enum.AGREE
		if err := tx.Save(&contactApply).Error; err != nil {
			zlog.Error("更新申请状态失败", zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}

		var existing model.UserContact
		if err := tx.Where("user_id = ? AND contact_id = ?", ownerId, contactId).
			First(&existing).Error; err == nil {
			tx.Rollback()
			return "你们已是好友，请勿重复添加", constants.BizCodeInvalid
		}

		now := time.Now()
		// 创建双方联系人关系
		if err := tx.Create(&model.UserContact{
			UserId:      ownerId,
			ContactId:   contactId,
			ContactType: contact_type_enum.USER,
			Status:      contact_status_enum.NORMAL,
			CreatedAt:   now,
			UpdatedAt:   now,
		}).Error; err != nil {
			zlog.Error("创建联系人关系失败 owner->contact", zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}

		if err := tx.Create(&model.UserContact{
			UserId:      contactId,
			ContactId:   ownerId,
			ContactType: contact_type_enum.USER,
			Status:      contact_status_enum.NORMAL,
			CreatedAt:   now,
			UpdatedAt:   now,
		}).Error; err != nil {
			zlog.Error("创建联系人关系失败 contact->owner", zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
		if err := tx.Commit().Error; err != nil {
			zlog.Error("事务提交失败", zap.Error(err))
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}

		// 清除缓存
		if err := myredis.DelKeyIfExists("contact_user_list_" + ownerId); err != nil {
			zlog.Error("删除联系人缓存失败", zap.Error(err))
		}
		if err := myredis.DelKeyIfExists("contact_user_list_" + contactId); err != nil {
			zlog.Error("删除联系人缓存失败", zap.Error(err))
		}

		return "已添加该联系人", constants.BizCodeSuccess
	}

	// 群聊类型处理
	var group model.GroupInfo
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "uuid = ?", ownerId).Error; err != nil {
		zlog.Error("查询群聊失败", zap.String("groupId", ownerId), zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if group.Status == group_status_enum.DISABLE {
		zlog.Info("群聊已被禁用", zap.String("groupId", ownerId))
		tx.Rollback()
		return "群聊已被禁用", constants.BizCodeInvalid
	}

	// 更新申请状态
	contactApply.Status = contact_apply_status_enum.AGREE
	if err := tx.Save(&contactApply).Error; err != nil {
		zlog.Error("更新申请状态失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	now := time.Now()
	// 创建群成员关系
	if err := tx.Create(&model.UserContact{
		UserId:      contactId,
		ContactId:   ownerId,
		ContactType: contact_type_enum.GROUP,
		Status:      contact_status_enum.NORMAL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		zlog.Error("创建加群联系人失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 判断用户是否已在群中（查 group_member）
	var exists int64
	if err := tx.Model(&model.GroupMember{}).
		Where("group_uuid = ? AND user_uuid = ?", ownerId, contactId).
		Count(&exists).Error; err != nil {
		zlog.Error("查询群成员失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if exists > 0 {
		zlog.Warn("用户已在群里", zap.String("userId", contactId), zap.String("groupId", ownerId))
		tx.Rollback()
		return "用户已在群里", constants.BizCodeInvalid
	}

	// 插入新成员
	member := model.GroupMember{
		GroupUuid: ownerId,
		UserUuid:  contactId,
		JoinedAt:  time.Now(),
	}
	if err := tx.Create(&member).Error; err != nil {
		zlog.Error("新增群成员失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	group.MemberCnt++
	if err := tx.Save(&group).Error; err != nil {
		zlog.Error("更新群成员失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	rsp := &respond.GetGroupInfoRespond{
		Uuid:      group.Uuid,
		Name:      group.Name,
		Notice:    group.Notice,
		Avatar:    group.Avatar,
		MemberCnt: group.MemberCnt,
		OwnerId:   group.OwnerId,
		AddMode:   group.AddMode,
		Status:    group.Status,
		IsDeleted: group.DeletedAt.Valid,
	}
	if err := myredis.SetCache("group_info_"+group.Uuid, rsp); err != nil {
		zlog.Warn("写入 redis 缓存失败", zap.Error(err))
	}

	if group.Status == group_status_enum.NORMAL {
		resp := respond.GetContactInfoRespond{
			ContactId:        group.Uuid,
			ContactName:      group.Name,
			ContactAvatar:    group.Avatar,
			ContactNotice:    group.Notice,
			ContactAddMode:   group.AddMode,
			ContactMemberCnt: group.MemberCnt,
			ContactOwnerId:   group.OwnerId,
		}
		if err := myredis.SetCache("contact_info_"+group.Uuid, &resp); err != nil {
			zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", group.Uuid), zap.Error(err))
		}
	} else {
		if err := myredis.DelKeyIfExists("contact_info_" + group.Uuid); err != nil {
			zlog.Error(err.Error())
		}
	}

	// 清缓存
	if err := myredis.DelKeyIfExists("my_joined_group_list_" + contactId); err != nil {
		zlog.Error("删除my_joined_group_list缓存失败", zap.Error(err))
	}

	if err := myredis.DelKeyIfExists("group_memberlist_" + group.Uuid); err != nil {
		zlog.Error(err.Error())
	}

	return "已通过加群申请", constants.BizCodeSuccess
}

// RefuseContactApply 拒绝联系人申请
// ownerId 如果是用户的话就是登录用户，如果是群聊的话就是群聊id
func (u *userContactService) RefuseContactApply(ownerId string, contactId string) (string, int) {
	if len(ownerId) == 0 || len(contactId) == 0 {
		return "非法参数", constants.BizCodeInvalid
	}

	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var contactApply model.ContactApply
	if err := tx.Where("contact_id = ? AND user_id = ?", ownerId, contactId).First(&contactApply).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return "申请记录不存在", constants.BizCodeInvalid
		}
		zlog.Error("查询申请记录失败", zap.String("ownerId", ownerId), zap.String("contactId", contactId), zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if contactApply.Status != contact_apply_status_enum.PENDING {
		tx.Rollback()
		return "该申请已处理过", constants.BizCodeInvalid
	}

	contactApply.Status = contact_apply_status_enum.REFUSE
	if err := tx.Save(&contactApply).Error; err != nil {
		zlog.Error("更新申请状态失败", zap.String("ownerId", ownerId), zap.String("contactId", contactId), zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := tx.Commit().Error; err != nil {
		zlog.Error("提交事务失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if ownerId[0] == 'U' {
		return "已拒绝该联系人申请", constants.BizCodeSuccess
	} else {
		return "已拒绝该加群申请", constants.BizCodeSuccess
	}
}

// BlackContact 拉黑联系人
func (u *userContactService) BlackContact(ownerId string, contactId string) (string, int) {
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	// 拉黑
	if res := tx.Model(&model.UserContact{}).Where("user_id = ? AND contact_id = ?", ownerId, contactId).Updates(map[string]interface{}{
		"status":     contact_status_enum.BLACK,
		"updated_at": time.Now(),
	}); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	// 被拉黑
	if res := tx.Model(&model.UserContact{}).Where("user_id = ? AND contact_id = ?", contactId, ownerId).Updates(map[string]interface{}{
		"status":     contact_status_enum.BE_BLACK,
		"updated_at": time.Now(),
	}); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	// 删除会话
	if err := tx.Where("send_id = ? AND receive_id = ?", ownerId, contactId).
		Delete(&model.Session{}).Error; err != nil {
		zlog.Error("删除会话失败 A->B", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := tx.Where("send_id = ? AND receive_id = ?", contactId, ownerId).
		Delete(&model.Session{}).Error; err != nil {
		zlog.Error("删除会话失败 B->A", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := myredis.DelKeyIfExists("contact_user_list_" + ownerId); err != nil {
		zlog.Error("删除联系人缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeyIfExists("contact_user_list_" + contactId); err != nil {
		zlog.Error("删除联系人缓存失败", zap.Error(err))
	}

	return "已拉黑该联系人", constants.BizCodeSuccess
}

// CancelBlackContact 取消拉黑联系人
func (u *userContactService) CancelBlackContact(ownerId string, contactId string) (string, int) {
	// 因为前端的设定，这里需要判断一下ownerId和contactId是不是有拉黑和被拉黑的状态
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var blackContact model.UserContact
	if err := tx.Where("user_id = ? AND contact_id = ?", ownerId, contactId).First(&blackContact).Error; err != nil {
		zlog.Error("查询拉黑关系失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if blackContact.Status != contact_status_enum.BLACK {
		tx.Rollback()
		return "未拉黑该联系人，无需解除拉黑", constants.BizCodeInvalid
	}

	var beBlackContact model.UserContact
	if err := tx.Where("user_id = ? AND contact_id = ?", contactId, ownerId).First(&beBlackContact).Error; err != nil {
		zlog.Error("查询被拉黑关系失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if beBlackContact.Status != contact_status_enum.BE_BLACK {
		tx.Rollback()
		return "该联系人未被拉黑，无需解除拉黑", constants.BizCodeInvalid
	}

	blackContact.Status = contact_status_enum.NORMAL
	beBlackContact.Status = contact_status_enum.NORMAL

	if err := tx.Save(&blackContact).Error; err != nil {
		zlog.Error("取消拉黑保存失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := tx.Save(&beBlackContact).Error; err != nil {
		zlog.Error("取消被拉黑保存失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := myredis.DelKeyIfExists("contact_user_list_" + ownerId); err != nil {
		zlog.Error("删除联系人缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeyIfExists("contact_user_list_" + contactId); err != nil {
		zlog.Error("删除联系人缓存失败", zap.Error(err))
	}
	return "已解除拉黑该联系人", constants.BizCodeSuccess
}

// BlackApply 拉黑一条申请
func (u *userContactService) BlackApply(ownerId string, contactId string) (string, int) {
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	var contactApply model.ContactApply
	if err := tx.Where("contact_id = ? AND user_id = ?", ownerId, contactId).First(&contactApply).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return "申请记录不存在", constants.BizCodeInvalid
		}
		zlog.Error("查询申请记录失败", zap.String("ownerId", ownerId), zap.String("contactId", contactId), zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if contactApply.Status == contact_apply_status_enum.BLACK {
		tx.Rollback()
		return "该申请已被拉黑", constants.BizCodeInvalid
	}

	contactApply.Status = contact_apply_status_enum.BLACK
	if err := tx.Save(&contactApply).Error; err != nil {
		zlog.Error("拉黑申请失败", zap.String("ownerId", ownerId), zap.String("contactId", contactId), zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	return "已拉黑该申请", constants.BizCodeSuccess
}
