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
	"github.com/afiff2/go-chat-server/pkg/enum/user_info/user_status_enum"
	myhash "github.com/afiff2/go-chat-server/pkg/util/hash"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userInfoService struct {
}

var UserInfoService = new(userInfoService)

// Login 登录，需要密码，不从redis查找
func (u *userInfoService) Login(loginReq request.LoginRequest) (string, *respond.GetUserInfoRespond, int) {
	var user model.UserInfo
	if res := dao.GormDB.First(&user, "telephone = ?", loginReq.Telephone); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			message := "用户不存在，请注册"
			zlog.Debug(message)
			return message, nil, constants.BizCodeInvalid
		}
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	// 使用哈希验证密码
	if !myhash.CheckPasswordHash(loginReq.Password, user.Password) {
		message := "密码不正确，请重试"
		zlog.Debug(message)
		return message, nil, constants.BizCodeInvalid
	}

	loginRsp := &respond.GetUserInfoRespond{
		Uuid:      user.Uuid,
		Telephone: user.Telephone,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Avatar:    user.Avatar,
		Gender:    user.Gender,
		Birthday:  user.Birthday,
		Signature: user.Signature,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		IsAdmin:   user.IsAdmin,
		Status:    user.Status,
	}
	// year, month, day := user.CreatedAt.Date()
	// loginRsp.CreatedAt = fmt.Sprintf("%d.%d.%d", year, month, day)
	// 将用户信息写入 Redis 缓存
	if err := myredis.SetCache("user_info_"+loginRsp.Uuid, loginRsp); err != nil {
		return "登陆成功, 写入 Redis 缓存失败", loginRsp, constants.BizCodeSuccess
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
	if err := myredis.SetCache("contact_info_"+loginRsp.Uuid, &resp); err != nil {
		zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", loginRsp.Uuid), zap.Error(err))
	}

	return "登陆成功", loginRsp, constants.BizCodeSuccess
}

// Register 注册，大概率改动数据库，不从redis查找
func (u *userInfoService) Register(registerReq request.RegisterRequest) (string, *respond.GetUserInfoRespond, int) {
	// 加密密码
	hashedPassword, err := myhash.HashPassword(registerReq.Password)
	if err != nil {
		zlog.Error("密码加密失败", zap.Error(err))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}

	tx := dao.GormDB.Begin()
	if tx.Error != nil {
		zlog.Error("开启事务失败", zap.Error(tx.Error))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	defer func() {
		if r := recover(); r != nil {
			zlog.Error("Register panic, rollback", zap.Any("recover", r))
			tx.Rollback()
		}
	}()

	// 不用校验手机号，前端校验
	// 判断电话是否已经被注册过了
	var user model.UserInfo
	if res := tx.Unscoped().
		Where("telephone = ?", registerReq.Telephone).
		First(&user); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			zlog.Debug("该电话不存在，可以注册")
		} else {
			zlog.Error(res.Error.Error())
			tx.Rollback()
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}
	} else { //要么软删状态，要么已存在
		if user.DeletedAt.Valid {
			// 软删状态 → “复活”这条记录
			user.DeletedAt.Valid = false
			user.DeletedAt.Time = time.Time{}    // 清空删除时间
			user.Password = hashedPassword       // 更新密码
			user.Nickname = registerReq.Nickname // 更新其它字段
			//user.Avatar = "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png"
			user.Status = user_status_enum.NORMAL

			if err := tx.Save(&user).Error; err != nil {
				zlog.Error("复活旧记录失败", zap.Error(err))
				tx.Rollback()
				return constants.SYSTEM_ERROR, nil, constants.BizCodeError
			}

			// 提交事务
			if err := tx.Commit().Error; err != nil {
				zlog.Error("事务提交失败", zap.Error(err))
				tx.Rollback()
				return constants.SYSTEM_ERROR, nil, constants.BizCodeError
			}

			// 构造返回 DTO
			registerRsp := &respond.GetUserInfoRespond{
				Uuid:      user.Uuid,
				Telephone: user.Telephone,
				Nickname:  user.Nickname,
				Email:     user.Email,
				Avatar:    user.Avatar,
				Gender:    user.Gender,
				Birthday:  user.Birthday,
				Signature: user.Signature,
				CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
				IsAdmin:   user.IsAdmin,
				Status:    user.Status,
			}
			if err := myredis.SetCache("user_info_"+registerRsp.Uuid, registerRsp); err != nil {
				zlog.Warn("写入 Redis 缓存失败", zap.Error(err))
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
			if err := myredis.SetCache("contact_info_"+registerRsp.Uuid, &resp); err != nil {
				zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", registerRsp.Uuid), zap.Error(err))
			}

			return "注册成功（恢复历史账号）", registerRsp, constants.BizCodeSuccess
		}
		// 正常存在且未删除
		zlog.Debug("该电话已经存在，注册失败")
		tx.Rollback()
		return "该电话已经存在，注册失败", nil, constants.BizCodeInvalid
	}

	//电话不存在，正常注册
	var newUser model.UserInfo
	newUser.Uuid = "U" + uuid.NewString()
	newUser.Telephone = registerReq.Telephone
	newUser.Password = hashedPassword
	newUser.Nickname = registerReq.Nickname
	newUser.Avatar = "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png"
	newUser.CreatedAt = time.Now()
	newUser.IsAdmin = 0
	newUser.Status = user_status_enum.NORMAL

	res := tx.Create(&newUser)
	if res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}

	// 注册成功，chat client建立
	//if err := chat.NewClientInit(c, newUser.Uuid); err != nil {
	//	return "", err
	//}
	registerRsp := &respond.GetUserInfoRespond{
		Uuid:      newUser.Uuid,
		Telephone: newUser.Telephone,
		Nickname:  newUser.Nickname,
		Email:     newUser.Email,
		Avatar:    newUser.Avatar,
		Gender:    newUser.Gender,
		Birthday:  newUser.Birthday,
		Signature: newUser.Signature,
		CreatedAt: newUser.CreatedAt.Format("2006-01-02 15:04:05"),
		IsAdmin:   newUser.IsAdmin,
		Status:    newUser.Status,
	}
	// year, month, day := newUser.CreatedAt.Date()
	// registerRsp.CreatedAt = fmt.Sprintf("%d.%d.%d", year, month, day)

	// 将用户信息写入 Redis 缓存
	if err := myredis.SetCache("user_info_"+registerRsp.Uuid, registerRsp); err != nil {
		zlog.Warn("写入 Redis 缓存失败", zap.Error(err))
	}

	resp := respond.GetContactInfoRespond{
		ContactId:        newUser.Uuid,
		ContactName:      newUser.Nickname,
		ContactAvatar:    newUser.Avatar,
		ContactBirthday:  newUser.Birthday,
		ContactEmail:     newUser.Email,
		ContactPhone:     newUser.Telephone,
		ContactGender:    newUser.Gender,
		ContactSignature: newUser.Signature,
	}
	if err := myredis.SetCache("contact_info_"+registerRsp.Uuid, &resp); err != nil {
		zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", registerRsp.Uuid), zap.Error(err))
	}

	return "注册成功", registerRsp, constants.BizCodeSuccess
}

// DeleteUsers 删除用户
// 用户是否启用禁用需要实时更新contact_user_list状态，所以redis的contact_user_list需要删除
func (u *userInfoService) DeleteUsers(uuidList []string) (string, int) {
	if len(uuidList) == 0 {
		return "无可处理用户", constants.BizCodeInvalid
	}

	// 收集所有受影响的群，用于事务提交后删除群缓存
	affectedGroups := make(map[string]struct{})

	// 数据库事务
	err := dao.GormDB.Transaction(func(tx *gorm.DB) error {
		//获取用户创建的群 & 加入的群
		var ownerGroups []model.GroupInfo
		if err := tx.Where("owner_id IN ?", uuidList).
			Find(&ownerGroups).Error; err != nil {
			return errors.New("查询群主群失败: " + err.Error())
		}

		var joinedMembers []model.GroupMember
		if err := tx.Where("user_uuid IN ?", uuidList).Find(&joinedMembers).Error; err != nil {
			return errors.New("查询群成员失败: " + err.Error())
		}

		ownerSet := make(map[string]struct{}, len(ownerGroups))
		for _, g := range ownerGroups {
			ownerSet[g.Uuid] = struct{}{}
			affectedGroups[g.Uuid] = struct{}{} // 无论后续是否解散，都需要清理群缓存
		}

		// 退群逻辑（用户是成员但不是群主）
		for _, m := range joinedMembers {
			if _, isOwner := ownerSet[m.GroupUuid]; isOwner {
				continue // 自己是群主，后面统一解散
			}
			affectedGroups[m.GroupUuid] = struct{}{}

			// 锁群信息，防止并发
			var group model.GroupInfo
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&group, "uuid = ?", m.GroupUuid).Error; err != nil {
				return errors.New("获取群聊失败: " + err.Error())
			}
			if group.OwnerId == m.UserUuid {
				continue // 理论上不会发生
			}

			// 校验成员存在并删除
			var gm model.GroupMember
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&gm, "group_uuid = ? AND user_uuid = ?", m.GroupUuid, m.UserUuid).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue // 不在群里，无需处理
				}
				return err
			}
			if err := tx.Delete(&model.GroupMember{}, "group_uuid = ? AND user_uuid = ?", m.GroupUuid, m.UserUuid).Error; err != nil {
				return err
			}

			// 更新群人数
			if group.MemberCnt > 0 {
				group.MemberCnt--
			}
			if err := tx.Save(&group).Error; err != nil {
				return err
			}

			// 群相关的会话 / 联系人 / 申请会在下面用户数据清理中被清理
		}

		//  解散群逻辑（用户是群主）
		for _, og := range ownerGroups {
			// 锁群信息
			var group model.GroupInfo
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&group, "uuid = ?", og.Uuid).Error; err != nil {
				return errors.New("群聊不存在: " + err.Error())
			}
			// 再次确认群主身份
			if group.OwnerId != og.OwnerId {
				continue
			}

			// 删除群成员（硬删）
			if err := tx.Unscoped().Where("group_uuid = ?", og.Uuid).Delete(&model.GroupMember{}).Error; err != nil {
				return err
			}

			// 删除群相关消息 / 会话 / 联系人 / 申请
			if err := tx.Where("receive_id = ?", og.Uuid).Delete(&model.Message{}).Error; err != nil {
				return err
			}
			if err := tx.Where("receive_id = ?", og.Uuid).Delete(&model.Session{}).Error; err != nil {
				return err
			}
			if err := tx.Where("contact_id = ?", og.Uuid).Delete(&model.UserContact{}).Error; err != nil {
				return err
			}
			if err := tx.Where("contact_id = ?", og.Uuid).Delete(&model.ContactApply{}).Error; err != nil {
				return err
			}

			// 软删群信息
			if err := tx.Delete(&group).Error; err != nil {
				return err
			}
		}

		// 删除与用户直接关联的数据
		if err := tx.Where("send_id IN ? OR receive_id IN ?", uuidList, uuidList).Delete(&model.Message{}).Error; err != nil {
			return errors.New("删除聊天消息失败: " + err.Error())
		}
		if err := tx.Where("send_id IN ? OR receive_id IN ?", uuidList, uuidList).Delete(&model.Session{}).Error; err != nil {
			return errors.New("删除会话失败: " + err.Error())
		}
		if err := tx.Where("user_id IN ? OR contact_id IN ?", uuidList, uuidList).Delete(&model.UserContact{}).Error; err != nil {
			return errors.New("删除联系人失败: " + err.Error())
		}
		if err := tx.Where("user_id IN ? OR contact_id IN ?", uuidList, uuidList).Delete(&model.ContactApply{}).Error; err != nil {
			return errors.New("删除申请记录失败: " + err.Error())
		}
		if err := tx.Where("uuid IN ?", uuidList).Delete(&model.UserInfo{}).Error; err != nil {
			return errors.New("删除用户失败: " + err.Error())
		}

		return nil // 事务正常提交
	})

	if err != nil {
		zlog.Error("DeleteUsers txn failed", zap.Error(err))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := myredis.DelKeysByUUIDList("user_info", uuidList); err != nil {
		zlog.Warn("删除用户缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeysByUUIDList("contact_info", uuidList); err != nil {
		zlog.Warn("删除联系人缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeysWithPrefix("contact_user_list"); err != nil {
		zlog.Error(err.Error())
	}
	//被删掉应该不会再访问这个user，等待redis自然清除
	// if err := DelKeysByUUIDList("contact_mygroup_list", uuidList); err != nil {
	// 	zlog.Warn("删除contact_user_list缓存失败", zap.Error(err))
	// }
	if err := myredis.DelKeysByPatternAndUUIDList("session_*", uuidList); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysByPrefixAndUUIDListWithSuffix("session", uuidList, "*"); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPrefix("session_list"); err != nil {
		zlog.Error(err.Error())
	}
	// 批量删除受影响群的缓存
	afg := make([]string, 0, len(affectedGroups))
	for k := range affectedGroups {
		afg = append(afg, k)
	}

	if err := myredis.DelKeysByUUIDList("group_memberlist", afg); err != nil {
		zlog.Warn("删除联系人缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeysByUUIDList("group_info", afg); err != nil {
		zlog.Warn("删除联系人缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeysByUUIDList("contact_info", afg); err != nil {
		zlog.Warn("删除联系人缓存失败", zap.Error(err))
	}
	return "删除用户成功", constants.BizCodeSuccess
}

// GetUserInfo 获取用户信息
func (u *userInfoService) GetUserInfo(uuid string) (string, *respond.GetUserInfoRespond, int) {
	// 优先查找redis
	zlog.Debug(uuid)
	cacheKey := "user_info_" + uuid
	rspString, err := myredis.GetKeyNilIsErr(cacheKey)
	if err != nil {
		//redis中没有
		if errors.Is(err, redis.Nil) {
			zlog.Debug("user_info 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("user_info 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
		}
		var user model.UserInfo
		if res := dao.GormDB.First(&user, "uuid = ?", uuid); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				zlog.Debug("该用户不存在，查找失败")
				return "该用户不存在，查找失败", nil, constants.BizCodeInvalid
			}
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
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

		if user.Status == user_status_enum.NORMAL {
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
			if err := myredis.SetCache("contact_info_"+uuid, &resp); err != nil {
				zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", uuid), zap.Error(err))
			}
		} else {
			if err := myredis.DelKeyIfExists("contact_info_" + uuid); err != nil {
				zlog.Error(err.Error())
			}
		}

		return "获取用户信息成功", &rsp, constants.BizCodeSuccess
	}
	//redis中有
	var rsp respond.GetUserInfoRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
		return "解析用户信息失败", nil, constants.BizCodeError
	}
	return "获取用户信息成功", &rsp, constants.BizCodeSuccess
}

// UpdateUserInfo 修改用户信息
// 某用户修改了信息，可能会影响contact_user_list，不需要删除redis的contact_user_list，timeout之后会自己更新
// 但是需要更新redis的user_info，因为可能影响用户搜索
func (u *userInfoService) UpdateUserInfo(updateReq request.UpdateUserInfoRequest) (string, int) {
	tx := dao.GormDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var user model.UserInfo
	if res := tx.First(&user, "uuid = ?", updateReq.Uuid); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			zlog.Debug("该用户不存在，修改失败")
			tx.Rollback()
			return "该用户不存在，修改失败", constants.BizCodeInvalid
		}
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if updateReq.Email != "" {
		user.Email = updateReq.Email
	}
	if updateReq.Nickname != "" {
		user.Nickname = updateReq.Nickname
	}
	if updateReq.Birthday != "" {
		user.Birthday = updateReq.Birthday
	}
	if updateReq.Signature != "" {
		user.Signature = updateReq.Signature
	}
	if updateReq.Avatar != "" {
		user.Avatar = updateReq.Avatar
	}
	if res := tx.Save(&user); res.Error != nil {
		zlog.Error(res.Error.Error())
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := tx.Commit().Error; err != nil {
		zlog.Error("事务提交失败", zap.Error(err))
		tx.Rollback()
		return constants.SYSTEM_ERROR, constants.BizCodeError
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

	// 更新用户信息成功后，检查并同步更新 contact_info 缓存
	if user.Status == user_status_enum.NORMAL {
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
		if err := myredis.SetCache("contact_info_"+updateReq.Uuid, &resp); err != nil {
			zlog.Warn("预写 contact_info 缓存失败", zap.String("contactId", updateReq.Uuid), zap.Error(err))
		}
	} else {
		if err := myredis.DelKeyIfExists("contact_info_" + updateReq.Uuid); err != nil {
			zlog.Error(err.Error())
		}
	}

	if err := myredis.DelKeysWithPrefix("contact_user_list"); err != nil {
		zlog.Error(err.Error())
	}

	return "修改用户信息成功", constants.BizCodeSuccess
}

// GetUserInfoList 获取用户列表除了ownerId之外 - 管理员
// 管理员少，而且如果用户更改了，那么管理员会一直频繁删除redis，更新redis，比较麻烦，所以管理员暂时不使用redis缓存
func (u *userInfoService) GetUserInfoList(ownerId string) (string, []respond.GetUserListRespond, int) {
	// redis中没有数据，从数据库中获取
	var users []model.UserInfo
	// 获取所有的用户(包括被软删除的)
	if res := dao.GormDB.Unscoped().Where("uuid != ?", ownerId).Find(&users); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	var rsp []respond.GetUserListRespond
	for _, user := range users {
		rp := respond.GetUserListRespond{
			Uuid:      user.Uuid,
			Telephone: user.Telephone,
			Nickname:  user.Nickname,
			Status:    user.Status,
			IsAdmin:   user.IsAdmin,
			IsDeleted: user.DeletedAt.Valid,
		}
		rsp = append(rsp, rp)
	}
	return "获取用户列表成功", rsp, constants.BizCodeSuccess
}

// AbleUsers 启用用户
// 用户是否启用/禁用需要实时更新 contact_user_list 状态，所以要删除对应的 Redis 缓存
func (u *userInfoService) AbleUsers(uuidList []string) (string, int) {
	// 一条 SQL 更新所有用户状态
	if res := dao.GormDB.
		Model(&model.UserInfo{}).
		Where("uuid IN ?", uuidList).
		Update("status", user_status_enum.NORMAL); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := myredis.DelKeysByUUIDList("user_info", uuidList); err != nil {
		zlog.Warn("删除用户缓存失败", zap.Error(err))
	}
	// 被禁用的不在缓存里，启用不用删除
	// if err := DelKeysByUUIDList("contact_info", uuidList); err != nil {
	// 	zlog.Warn("删除联系人缓存失败", zap.Error(err))
	// }
	// 删除所有 "contact_user_list" 开头的 key
	if err := myredis.DelKeysWithPrefix("contact_user_list"); err != nil {
		zlog.Error(err.Error())
	}

	return "启用用户成功", constants.BizCodeSuccess
}

// DisableUsers 禁用用户
// 用户启用/禁用需要实时更新 contact_user_list 状态，所以要删除对应的 Redis 缓存
func (u *userInfoService) DisableUsers(uuidList []string) (string, int) {
	err := dao.GormDB.Transaction(func(tx *gorm.DB) error {
		// 将用户状态置为 DISABLE
		if res := tx.
			Model(&model.UserInfo{}).
			Where("uuid IN ?", uuidList).
			Update("status", user_status_enum.DISABLE); res.Error != nil {
			zlog.Error("批量禁用用户状态失败", zap.Error(res.Error))
			return res.Error
		}
		//软删除所有相关会话
		if res := tx.
			Where("send_id IN ? OR receive_id IN ?", uuidList, uuidList).
			Delete(&model.Session{}); res.Error != nil {
			zlog.Error("批量软删除会话失败", zap.Error(res.Error))
			return res.Error
		}
		// 返回 nil，事务会自动提交
		return nil
	})
	if err != nil {
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := myredis.DelKeysByUUIDList("user_info", uuidList); err != nil {
		zlog.Warn("删除用户缓存失败", zap.Error(err))
	}
	if err := myredis.DelKeysByUUIDList("contact_info", uuidList); err != nil {
		zlog.Warn("删除联系人缓存失败", zap.Error(err))
	}
	// 删除所有 "contact_user_list" 前缀的 Redis key
	if err := myredis.DelKeysWithPrefix("contact_user_list"); err != nil {
		zlog.Error(err.Error())
	}

	return "禁用用户成功", constants.BizCodeSuccess
}

// SetAdmin 设置管理员
func (u *userInfoService) SetAdmin(uuidList []string, isAdmin int8) (string, int) {
	// 一条 SQL 把所有指定 UUID 的用户 is_admin 字段更新为 isAdmin
	if res := dao.GormDB.
		Model(&model.UserInfo{}).
		Where("uuid IN ?", uuidList).
		Update("is_admin", isAdmin); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	if err := myredis.DelKeysByUUIDList("user_info", uuidList); err != nil {
		zlog.Warn("删除用户缓存失败", zap.Error(err))
	}

	return "设置管理员成功", constants.BizCodeSuccess
}
