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
	"github.com/afiff2/go-chat-server/pkg/enum/contact/contact_type_enum"
	"github.com/afiff2/go-chat-server/pkg/enum/group_info/group_status_enum"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type groupInfoService struct {
}

var GroupInfoService = new(groupInfoService)

// CreateGroup 创建群聊
func (g *groupInfoService) CreateGroup(groupReq request.CreateGroupRequest) (string, int) {
	// 开启事务
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("CreateGroup panic, rollback", zap.Any("recover", r))
			tx.Rollback()
		}
	}()

	group := model.GroupInfo{
		Uuid:      "G" + uuid.NewString(),
		Name:      groupReq.Name,
		Notice:    groupReq.Notice,
		OwnerId:   groupReq.OwnerId,
		MemberCnt: 1,
		AddMode:   groupReq.AddMode,
		Avatar:    groupReq.Avatar,
		Status:    group_status_enum.NORMAL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if res := tx.Create(&group); res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	//添加自己作为群员
	gm := model.GroupMember{
		GroupUuid: group.Uuid,
		UserUuid:  groupReq.OwnerId,
		JoinedAt:  time.Now(),
	}
	if res := tx.Create(&gm); res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 添加联系人
	contact := model.UserContact{
		UserId:      groupReq.OwnerId,
		ContactId:   group.Uuid,
		ContactType: contact_type_enum.GROUP,
		Status:      contact_status_enum.NORMAL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if res := tx.Create(&contact); res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 写入 group_info_{groupId} 缓存
	groupInfoRsp := respond.GetGroupInfoRespond{
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
	if err := myredis.SetCache("group_info_"+group.Uuid, &groupInfoRsp); err != nil {
		zlog.Warn("预写 group_info 缓存失败", zap.String("groupId", group.Uuid), zap.Error(err))
	}

	// 同步更新 contact_info 缓存
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

	rsp2, err2 := myredis.GetKeyNilIsErr("my_joined_group_list_" + groupReq.OwnerId)
	if err2 == nil {
		// 如果已有缓存，追加新群
		var joined []respond.LoadMyGroupRespond
		if err := json.Unmarshal([]byte(rsp2), &joined); err != nil {
			zlog.Warn("反序列化 my_joined_group_list 缓存失败，跳过更新", zap.Error(err))
		} else {
			joined = append(joined, respond.LoadMyGroupRespond{
				GroupId:   group.Uuid,
				GroupName: group.Name,
				Avatar:    group.Avatar,
			})
			if err := myredis.SetCache("my_joined_group_list_"+groupReq.OwnerId, &joined); err != nil {
				zlog.Warn("更新 Redis 缓存失败", zap.Error(err))
			}
		}
	} else if !errors.Is(err2, redis.Nil) {
		zlog.Warn("读取 Redis my_joined_group_list 缓存失败", zap.Error(err2))
	}

	rsp3, err3 := myredis.GetKeyNilIsErr("contact_mygroup_list_" + groupReq.OwnerId)
	if err3 == nil {
		// 如果已有缓存，追加新群
		var owned []respond.LoadMyGroupRespond
		if err := json.Unmarshal([]byte(rsp3), &owned); err != nil {
			zlog.Warn("反序列化 contact_mygroup_list 缓存失败，跳过更新", zap.Error(err))
		} else {
			owned = append(owned, respond.LoadMyGroupRespond{
				GroupId:   group.Uuid,
				GroupName: group.Name,
				Avatar:    group.Avatar,
			})
			if err := myredis.SetCache("contact_mygroup_list_"+groupReq.OwnerId, &owned); err != nil {
				zlog.Warn("更新 Redis 缓存失败", zap.Error(err))
			}
		}
	} else if !errors.Is(err3, redis.Nil) {
		zlog.Warn("读取 Redis contact_mygroup_list 缓存失败", zap.Error(err3))
	}

	return "创建成功", constants.BizCodeSuccess
}

// LoadMyGroup 获取我创建的群聊
func (g *groupInfoService) LoadMyGroup(ownerId string) (string, []respond.LoadMyGroupRespond, int) {
	cacheKey := "contact_mygroup_list_" + ownerId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		//redis中没有
		if errors.Is(err, redis.Nil) {
			zlog.Debug("contact_mygroup_list 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("contact_mygroup_list 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
		var groupList []model.GroupInfo
		if res := dao.GormDB.Order("created_at DESC").Where("owner_id = ? AND status = ?", ownerId, group_status_enum.NORMAL).Find(&groupList); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}
		var groupListRsp []respond.LoadMyGroupRespond

		for _, group := range groupList {
			//contact_mygroup_list_{ownerId} 缓存
			groupListRsp = append(groupListRsp, respond.LoadMyGroupRespond{
				GroupId:   group.Uuid,
				GroupName: group.Name,
				Avatar:    group.Avatar,
			})

			//group_info_{groupId} 缓存
			groupInfoRsp := respond.GetGroupInfoRespond{
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
			if err := myredis.SetCache("group_info_"+group.Uuid, &groupInfoRsp); err != nil {
				zlog.Warn("预写 group_info 缓存失败", zap.String("groupId", group.Uuid), zap.Error(err))
			}

			// 同步更新 contact_info 缓存
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
		}

		// 写入 contact_mygroup_list_{ownerId} 缓存
		if err := myredis.SetCache("contact_mygroup_list_"+ownerId, &groupListRsp); err != nil {
			zlog.Warn("写入 Redis 缓存失败", zap.Error(err))
		}
		return "获取成功", groupListRsp, constants.BizCodeSuccess
	}
	//redis中有
	var groupListRsp []respond.LoadMyGroupRespond
	if err := json.Unmarshal([]byte(rspString), &groupListRsp); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	return "获取成功", groupListRsp, constants.BizCodeSuccess
}

// GetGroupInfo 获取群聊详情
func (g *groupInfoService) GetGroupInfo(groupId string) (string, *respond.GetGroupInfoRespond, int) {
	cacheKey := "group_info_" + groupId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		//redis中没有
		if errors.Is(err, redis.Nil) {
			zlog.Debug("group_info 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("group_info 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
		var group model.GroupInfo
		if res := dao.GormDB.First(&group, "uuid = ?", groupId); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
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
		if err := myredis.SetCache("group_info_"+groupId, rsp); err != nil {
			zlog.Warn("写入 redis 缓存失败", zap.Error(err))
		}
		// 同步更新 contact_info 缓存
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
			if err := myredis.SetCache("contact_info_"+groupId, &resp); err != nil {
				zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", groupId), zap.Error(err))
			}
		} else {
			if err := myredis.DelKeyIfExists("contact_info_" + groupId); err != nil {
				zlog.Error(err.Error())
			}
		}
		return "获取成功", rsp, constants.BizCodeSuccess
	}
	//redis中有
	zlog.Debug("Redis 缓存命中", zap.String("key", cacheKey))
	var rsp respond.GetGroupInfoRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error("Redis数据解析失败", zap.Error(err))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	return "获取成功", &rsp, constants.BizCodeSuccess
}

// GetGroupInfoList 获取群聊列表 - 管理员
// 管理员请求频次低，缓存更新代价高，不缓存可以减少复杂度
func (g *groupInfoService) GetGroupInfoList() (string, []respond.GetGroupListRespond, int) {
	var groupList []model.GroupInfo
	//包括已逻辑删除的
	if res := dao.GormDB.Unscoped().Find(&groupList); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	var rsp []respond.GetGroupListRespond
	for _, group := range groupList {
		rp := respond.GetGroupListRespond{
			Uuid:      group.Uuid,
			Name:      group.Name,
			OwnerId:   group.OwnerId,
			Status:    group.Status,
			IsDeleted: group.DeletedAt.Valid,
		}
		rsp = append(rsp, rp)
	}
	zlog.Info("获取群聊列表成功")
	return "获取成功", rsp, constants.BizCodeSuccess
}

// LeaveGroup 退群，redis中信息不够，过多操作加入事务
func (g *groupInfoService) LeaveGroup(userId string, groupId string) (string, int) {
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("事务 panic 回滚", zap.Any("recover", r))
			tx.Rollback()
		}
	}()
	// 加行锁防并发覆盖写
	var group model.GroupInfo
	if res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "uuid = ?", groupId); res.Error != nil {
		zlog.Error("获取群聊失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if group.OwnerId == userId {
		tx.Rollback()
		return "群主不允许主动退出群聊", constants.BizCodeInvalid
	}

	var gm model.GroupMember
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&gm, "group_uuid = ? AND user_uuid = ?", groupId, userId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 处理找不到的情况
			tx.Rollback()
			return "成员信息异常，用户不在群中", constants.BizCodeInvalid
		} else {
			// 处理其他数据库错误
			zlog.Error("数据库错误", zap.Error(err))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
	}

	if err := tx.Delete(&model.GroupMember{},
		"group_uuid = ? AND user_uuid = ?", groupId, userId).Error; err != nil {
		zlog.Error("删除群员信息失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if group.MemberCnt > 0 {
		group.MemberCnt-- //防御性编程
	}
	if res := tx.Save(&group); res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除会话
	if res := tx.Where("send_id = ? AND receive_id = ?", userId, groupId).
		Delete(&model.Session{}); res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	// 删除联系人
	if res := tx.Where("user_id = ? AND contact_id = ?", userId, groupId).
		Delete(&model.UserContact{}); res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	// 删除申请记录，后面还可以加
	if res := tx.Where("contact_id = ? AND user_id = ?", groupId, userId).
		Delete(&model.ContactApply{}); res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		//  commit 之后 rollback 已无意义
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	//人数变化更新缓存
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
	if err := myredis.DelKeyIfExists("group_memberlist_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.SetCache("group_info_"+groupId, rsp); err != nil {
		zlog.Warn("写入 redis 缓存失败", zap.Error(err))
	}
	// 同步更新 contact_info 缓存
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
		if err := myredis.SetCache("contact_info_"+groupId, &resp); err != nil {
			zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", groupId), zap.Error(err))
		}
	} else {
		if err := myredis.DelKeyIfExists("contact_info_" + groupId); err != nil {
			zlog.Error(err.Error())
		}
	}
	if err := myredis.DelKeyIfExists("group_session_list_" + userId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeyIfExists("my_joined_group_list_" + userId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeyIfExists("session_" + userId + "_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	zlog.Info("用户退群成功", zap.String("userId", userId), zap.String("groupId", groupId))
	return "退群成功", constants.BizCodeSuccess
}

// DismissGroup 解散群聊
func (g *groupInfoService) DismissGroup(ownerId, groupId string) (string, int) {
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("事务 panic 回滚", zap.Any("recover", r))
			tx.Rollback()
		}
	}()
	// 查询
	var group model.GroupInfo
	if res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "uuid = ?", groupId); res.Error != nil {
		zlog.Error("群聊不存在", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if group.OwnerId != ownerId {
		tx.Rollback()
		return "只有群主才能解散群聊", constants.BizCodeInvalid
	}

	// 物理删除 group_member
	if res := tx.
		Where("group_uuid = ?", groupId).
		Unscoped(). // 直接硬删（因为 group_member 没有 DeletedAt）
		Delete(&model.GroupMember{}); res.Error != nil {
		zlog.Error("删除群成员失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if res := tx.Where("receive_id = ?", groupId).Delete(&model.Message{}); res.Error != nil {
		zlog.Error("删除聊天消息失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除相关会话
	if res := tx.Where("receive_id = ?", groupId).Delete(&model.Session{}); res.Error != nil {
		zlog.Error("删除群会话失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	// 删除相关联系人（软删）
	if res := tx.Where("contact_id = ?", groupId).Delete(&model.UserContact{}); res.Error != nil {
		zlog.Error("删除用户联系人失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	// 删除群申请记录（软删）
	if res := tx.Where("contact_id = ?", groupId).Delete(&model.ContactApply{}); res.Error != nil {
		zlog.Error("删除申请记录失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除 GroupInfo（软删）
	if res := tx.Delete(&group); res.Error != nil {
		zlog.Error("解散群聊失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := myredis.DelKeyIfExists("group_info_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeyIfExists("contact_info_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeyIfExists("contact_mygroup_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPrefix("group_session_list"); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPrefix("my_joined_group_list"); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("session_*_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeyIfExists("group_memberlist_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	return "解散群聊成功", constants.BizCodeSuccess
}

// DeleteGroups 删除列表中群聊 - 管理员
func (g *groupInfoService) DeleteGroupsByAdmin(uuidList []string) (string, int) {
	if len(uuidList) == 0 {
		return "无可删除群聊", constants.BizCodeInvalid
	}

	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("DeleteGroups panic 回滚", zap.Any("recover", r))
			tx.Rollback()
		}
	}()

	// 检查是否存在
	var groups []model.GroupInfo
	if res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("uuid IN ?", uuidList).Find(&groups); res.Error != nil {
		zlog.Error("获取群聊失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if len(groups) != len(uuidList) {
		tx.Rollback()
		return "部分群聊不存在或已删除", constants.BizCodeInvalid
	}
	// 记录所有涉及到的 ownerId（去重）
	ownerIdSet := make(map[string]struct{})
	for _, g := range groups {
		ownerIdSet[g.OwnerId] = struct{}{}
	}

	// 物理删除 group_member
	if res := tx.
		Where("group_uuid IN ?", uuidList).
		Unscoped(). // 直接硬删（因为 group_member 没有 DeletedAt）
		Delete(&model.GroupMember{}); res.Error != nil {
		zlog.Error("删除群成员失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除会话
	if res := tx.Where("receive_id IN ?", uuidList).Delete(&model.Session{}); res.Error != nil {
		zlog.Error("删除群会话失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除联系人
	if res := tx.Where("contact_id IN ?", uuidList).Delete(&model.UserContact{}); res.Error != nil {
		zlog.Error("删除联系人失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除申请记录
	if res := tx.Where("contact_id IN ?", uuidList).Delete(&model.ContactApply{}); res.Error != nil {
		zlog.Error("删除申请记录失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 删除群信息（GORM软删除）
	if res := tx.Where("uuid IN ?", uuidList).Delete(&model.GroupInfo{}); res.Error != nil {
		zlog.Error("删除群信息失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := myredis.DelKeysByUUIDList("group_info", uuidList); err != nil {
		zlog.Warn("删除group_info缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeysByUUIDList("contact_info", uuidList); err != nil {
		zlog.Warn("删除contact_info缓存失败", zap.Error(err))
	}
	// 删除相关 owner 的 contact_mygroup_list_{ownerId}
	for ownerId := range ownerIdSet {
		cacheKey := "contact_mygroup_list_" + ownerId
		if err := myredis.DelKeyIfExists(cacheKey); err != nil {
			zlog.Error("删除 owner 群聊缓存失败", zap.String("ownerId", ownerId), zap.Error(err))
		}
	}
	if err := myredis.DelKeysWithPrefix("group_session_list"); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPrefix("my_joined_group_list"); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysByPatternAndUUIDList("session_*", uuidList); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysByUUIDList("group_memberlist", uuidList); err != nil {
		zlog.Error(err.Error())
	}
	return "解散/删除群聊成功", constants.BizCodeSuccess
}

// CheckGroupAddMode 检查群聊加群方式
func (g *groupInfoService) CheckGroupAddMode(groupId string) (string, int8, int) {
	rspString, err := myredis.GetKeyNilIsErr("group_info_" + groupId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("group_info 缓存未命中，回库读取", zap.String("key", "group_info_"+groupId))
		} else {
			zlog.Warn("group_info 读取发生错误，回库读取", zap.Error(err), zap.String("key", "group_info_"+groupId))
		}
		var group model.GroupInfo
		if res := dao.GormDB.First(&group, "uuid = ?", groupId); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1, constants.BizCodeError
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
		if err := myredis.SetCache("group_info_"+groupId, rsp); err != nil {
			zlog.Warn("写入 redis 缓存失败", zap.Error(err))
		}
		// 同步更新 contact_info 缓存
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
			if err := myredis.SetCache("contact_info_"+groupId, &resp); err != nil {
				zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", groupId), zap.Error(err))
			}
		} else {
			if err := myredis.DelKeyIfExists("contact_info_" + groupId); err != nil {
				zlog.Error(err.Error())
			}
		}
		return "加群方式获取成功", group.AddMode, constants.BizCodeSuccess
	}
	var rsp respond.GetGroupInfoRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error("group_info Redis 数据反序列化失败", zap.Error(err))
		return constants.SYSTEM_ERROR, -1, constants.BizCodeError
	}
	return "加群方式获取成功", rsp.AddMode, constants.BizCodeSuccess
}

// EnterGroupDirectly 直接进群
func (g *groupInfoService) EnterGroupDirectly(groupId, userId string) (string, int) {
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("EnterGroupDirectly panic 回滚", zap.Any("recover", r))
			tx.Rollback()
		}
	}()
	// 查询群聊
	var group model.GroupInfo
	if res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "uuid = ?", groupId); res.Error != nil {
		zlog.Error("查询群聊失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 判断用户是否已在群中（查 group_member）
	var exists int64
	if err := tx.Model(&model.GroupMember{}).
		Where("group_uuid = ? AND user_uuid = ?", groupId, userId).
		Count(&exists).Error; err != nil {
		zlog.Error("查询群成员失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if exists > 0 {
		zlog.Warn("用户已在群里", zap.String("userId", userId), zap.String("groupId", groupId))
		tx.Rollback()
		return "用户已在群里", constants.BizCodeInvalid
	}
	// 插入新成员
	member := model.GroupMember{
		GroupUuid: groupId,
		UserUuid:  userId,
		JoinedAt:  time.Now(),
	}
	if err := tx.Create(&member).Error; err != nil {
		zlog.Error("新增群成员失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	group.MemberCnt++
	// 保存群信息
	if res := tx.Save(&group); res.Error != nil {
		zlog.Error("更新群信息失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	newContact := model.UserContact{
		UserId:      userId,
		ContactId:   groupId,
		ContactType: contact_type_enum.GROUP,    // 群聊
		Status:      contact_status_enum.NORMAL, // 正常
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if res := tx.Create(&newContact); res.Error != nil {
		zlog.Error("创建联系人失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	// 提交事务
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
	if err := myredis.SetCache("group_info_"+groupId, rsp); err != nil {
		zlog.Warn("写入 redis 缓存失败", zap.Error(err))
	}
	// 同步更新 contact_info 缓存
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
		if err := myredis.SetCache("contact_info_"+groupId, &resp); err != nil {
			zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", groupId), zap.Error(err))
		}
	} else {
		if err := myredis.DelKeyIfExists("contact_info_" + groupId); err != nil {
			zlog.Error(err.Error())
		}
	}
	// if err := myredis.DelKeyIfExists("group_session_list_" + userId); err != nil {
	// 	zlog.Error(err.Error())
	// }
	if err := myredis.DelKeyIfExists("my_joined_group_list_" + userId); err != nil {
		zlog.Error(err.Error())
	}
	//if err := myredis.DelKeyIfExists("session_" + ownerId + "_" + contactId); err != nil {
	//	zlog.Error(err.Error())
	//}
	if err := myredis.DelKeyIfExists("group_memberlist_" + groupId); err != nil {
		zlog.Error(err.Error())
	}

	return "进群成功", constants.BizCodeSuccess
}

// SetGroupsStatus 设置群聊是否启用
func (g *groupInfoService) SetGroupsStatus(uuidList []string, status int8) (string, int) {
	if len(uuidList) == 0 {
		return "无可处理群聊", constants.BizCodeInvalid
	}

	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("SetGroupsStatus panic", zap.Any("recover", r))
			tx.Rollback()
		}
	}()

	// 批量更新群聊状态
	if res := tx.Model(&model.GroupInfo{}).
		Where("uuid IN ?", uuidList).
		Update("status", status); res.Error != nil {
		zlog.Error("更新群状态失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 如果禁用群聊，批量删除会话记录
	if status == group_status_enum.DISABLE {
		if res := tx.Where("receive_id IN ?", uuidList).
			Delete(&model.Session{}); res.Error != nil {
			zlog.Error("批量删除会话失败", zap.Error(res.Error))
			tx.Rollback()
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
	}

	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := myredis.DelKeysByUUIDList("group_info", uuidList); err != nil {
		zlog.Warn("删除group_info缓存失败", zap.Error(err))
	}

	if status == group_status_enum.DISABLE {
		if err := myredis.DelKeysByUUIDList("contact_info", uuidList); err != nil {
			zlog.Warn("删除contact_info缓存失败", zap.Error(err))
		}
		if err := myredis.DelKeysByUUIDList("my_joined_group_list", uuidList); err != nil {
			zlog.Warn("删除my_joined_group_list缓存失败", zap.Error(err))
		}
		if err := myredis.DelKeysByUUIDList("contact_mygroup_list", uuidList); err != nil {
			zlog.Warn("删除contact_mygroup_list缓存失败", zap.Error(err))
		}
	}

	// group_session_list简单，内部不会修改
	// if err := myredis.DelKeysWithPrefix("group_session_list"); err != nil {
	// 	zlog.Error(err.Error())
	// }

	if err := myredis.DelKeysByPatternAndUUIDList("session_*", uuidList); err != nil {
		zlog.Error(err.Error())
	}

	return "设置成功", constants.BizCodeSuccess
}

// UpdateGroupInfo 更新群聊信息
func (g *groupInfoService) UpdateGroupInfo(req request.UpdateGroupInfoRequest) (string, int) {
	var group model.GroupInfo
	if res := dao.GormDB.First(&group, "uuid = ?", req.Uuid); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if req.Name != "" {
		group.Name = req.Name
	}
	if req.AddMode != -1 {
		group.AddMode = req.AddMode
	}
	if req.Notice != "" {
		group.Notice = req.Notice
	}
	if req.Avatar != "" {
		group.Avatar = req.Avatar
	}
	if res := dao.GormDB.Save(&group); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	// 修改会话
	var sessionList []model.Session
	if res := dao.GormDB.Where("receive_id = ?", req.Uuid).Find(&sessionList); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	for _, session := range sessionList {
		session.ReceiveName = group.Name
		session.Avatar = group.Avatar
		if res := dao.GormDB.Save(&session); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
	}

	// 写入 group_info_{groupId} 缓存
	groupInfoRsp := respond.GetGroupInfoRespond{
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
	if err := myredis.SetCache("group_info_"+group.Uuid, &groupInfoRsp); err != nil {
		zlog.Warn("预写 group_info 缓存失败", zap.String("groupId", group.Uuid), zap.Error(err))
	}

	// 同步更新 contact_info 缓存
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

	if err := myredis.DelKeysWithPrefix("my_joined_group_list"); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeyIfExists("contact_mygroup_list_" + group.OwnerId); err != nil {
		zlog.Error("删除 contact_mygroup_list 缓存失败", zap.String("ownerId", group.OwnerId), zap.Error(err))
	}
	if err := myredis.DelKeysWithPrefix("group_session_list"); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("session_*_" + group.Uuid); err != nil {
		zlog.Error(err.Error())
	}
	return "更新成功", constants.BizCodeSuccess
}

// GetGroupMemberList 获取群聊成员列表
func (g *groupInfoService) GetGroupMemberList(groupId string) (string, []respond.GetGroupMemberListRespond, int) {
	cacheKey := "group_memberlist_" + groupId
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Debug("group_memberlist 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("group_memberlist 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
		var group model.GroupInfo
		if res := dao.GormDB.First(&group, "uuid = ?", groupId); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return "群聊不存在", nil, constants.BizCodeInvalid
			}
			zlog.Error("查询群信息失败", zap.Error(res.Error))
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}

		var members []model.GroupMember
		if res := dao.GormDB.
			Where("group_uuid = ?", groupId).
			Find(&members); res.Error != nil {
			zlog.Error("查询群成员失败", zap.Error(res.Error))
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}

		memberIds := make([]string, len(members))
		for i, m := range members {
			memberIds[i] = m.UserUuid
		}

		// 一次性查出所有用户信息
		var users []model.UserInfo
		if res := dao.GormDB.Where("uuid IN ?", memberIds).Find(&users); res.Error != nil {
			zlog.Error("批量查询用户信息失败", zap.Error(res.Error))
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}

		var rspList []respond.GetGroupMemberListRespond
		for _, user := range users {
			rspList = append(rspList, respond.GetGroupMemberListRespond{
				UserId:   user.Uuid,
				Nickname: user.Nickname,
				Avatar:   user.Avatar,
			})
		}
		if err := myredis.SetCache("group_memberlist_"+groupId, &rspList); err != nil {
			zlog.Warn("写入成员列表缓存失败", zap.Error(err))
		}

		return "获取群聊成员列表成功", rspList, constants.BizCodeSuccess
	}
	// Redis 命中
	var rsp []respond.GetGroupMemberListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error("成员缓存反序列化失败", zap.Error(err))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}

	return "获取群聊成员列表成功", rsp, constants.BizCodeSuccess
}

// RemoveGroupMembers 移除群聊成员
func (g *groupInfoService) RemoveGroupMembers(req request.RemoveGroupMembersRequest) (string, int) {
	// 开启事务
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("RemoveGroupMembers panic 回滚", zap.Any("recover", r))
			tx.Rollback()
		}
	}()

	// 查询群信息
	var group model.GroupInfo
	if res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "uuid = ?", req.GroupId); res.Error != nil {
		zlog.Error("获取群聊信息失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 3. 拉取现有成员 → map
	var members []string
	if err := tx.Model(&model.GroupMember{}).
		Where("group_uuid = ?", req.GroupId).
		Pluck("user_uuid", &members).Error; err != nil {

		zlog.Error("查询群成员失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 处理移除逻辑
	memberSet := make(map[string]struct{}, len(members))
	for _, m := range members {
		memberSet[m] = struct{}{}
	}

	//构造待移除成员集合（过滤群主）
	toDelete := make([]string, 0, len(req.UuidList))
	for _, uuid := range req.UuidList {
		if uuid == req.OwnerId {
			tx.Rollback()
			return "不能移除群主", constants.BizCodeInvalid
		}
		if _, ok := memberSet[uuid]; !ok {
			zlog.Warn("试图移除不存在的群成员", zap.String("groupId", req.GroupId), zap.String("userId", uuid))
			continue
		}
		toDelete = append(toDelete, uuid)
		delete(memberSet, uuid)
		if group.MemberCnt > 0 {
			group.MemberCnt--
		}
	}

	if len(toDelete) == 0 {
		tx.Rollback()
		return "无可移除的成员", constants.BizCodeInvalid
	}

	// 批量删除 group_member 记录
	if err := tx.Where("group_uuid = ? AND user_uuid IN ?", req.GroupId, toDelete).
		Delete(&model.GroupMember{}).Error; err != nil {

		zlog.Error("删除 group_member 失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 批量删除 Session
	if res := tx.Where("send_id IN ? AND receive_id = ?", toDelete, req.GroupId).
		Delete(&model.Session{}); res.Error != nil {
		zlog.Error("批量删除会话失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	//批量删除联系人
	if res := tx.Where("user_id IN ? AND contact_id = ?", toDelete, req.GroupId).
		Delete(&model.UserContact{}); res.Error != nil {
		zlog.Error("批量删除联系人失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	//批量删除申请记录
	if res := tx.Where("user_id IN ? AND contact_id = ?", toDelete, req.GroupId).
		Delete(&model.ContactApply{}); res.Error != nil {
		zlog.Error("批量删除申请记录失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 更新群组信息
	if res := tx.Save(&group); res.Error != nil {
		zlog.Error("保存群信息失败", zap.Error(res.Error))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 5. 提交事务
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	// 6. 缓存清理：只清除被移除用户的缓存
	for _, uuid := range toDelete {
		if err := myredis.DelKeyIfExists("group_session_list_" + uuid); err != nil {
			zlog.Error(err.Error())
		}
		if err := myredis.DelKeyIfExists("my_joined_group_list_" + uuid); err != nil {
			zlog.Error(err.Error())
		}
		if err := myredis.DelKeyIfExists("session_" + uuid + "_" + req.GroupId); err != nil {
			zlog.Error(err.Error())
		}
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

	// 同步更新 contact_info 缓存
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

	if err := myredis.DelKeyIfExists("group_memberlist_" + req.GroupId); err != nil {
		zlog.Error(err.Error())
	}
	return "移除群聊成员成功", constants.BizCodeSuccess
}
