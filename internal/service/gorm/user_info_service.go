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
	"github.com/afiff2/go-chat-server/pkg/enum/user_status_enum"
	myhash "github.com/afiff2/go-chat-server/pkg/util/hash"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"gorm.io/gorm"
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
	if err := SetCache("user_info", loginRsp.Uuid, loginRsp); err != nil {
		return "登陆成功, 写入 Redis 缓存失败", loginRsp, constants.BizCodeSuccess
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
	// 不用校验手机号，前端校验
	// 判断电话是否已经被注册过了
	var user model.UserInfo
	if res := dao.GormDB.Unscoped().
		Where("telephone = ?", registerReq.Telephone).
		First(&user); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			zlog.Debug("该电话不存在，可以注册")
		} else {
			zlog.Error(res.Error.Error())
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

			if err := dao.GormDB.Save(&user).Error; err != nil {
				zlog.Error("复活旧记录失败", zap.Error(err))
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
			if err := SetCache("user_info", registerRsp.Uuid, registerRsp); err != nil {
				zlog.Warn("写入 Redis 缓存失败", zap.Error(err))
			}
			return "注册成功（恢复历史账号）", registerRsp, constants.BizCodeSuccess
		}
		// 正常存在且未删除
		zlog.Debug("该电话已经存在，注册失败")
		return "该电话已经存在，注册失败", nil, constants.BizCodeInvalid
	}

	//电话不存在，正常注册
	var newUser model.UserInfo
	newUser.Uuid = uuid.NewString()
	newUser.Telephone = registerReq.Telephone
	newUser.Password = hashedPassword
	newUser.Nickname = registerReq.Nickname
	newUser.Avatar = "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png"
	newUser.CreatedAt = time.Now()
	newUser.IsAdmin = 0
	newUser.Status = user_status_enum.NORMAL

	res := dao.GormDB.Create(&newUser)
	if res.Error != nil {
		zlog.Error(res.Error.Error())
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
	if err := SetCache("user_info", registerRsp.Uuid, registerRsp); err != nil {
		zlog.Warn("写入 Redis 缓存失败", zap.Error(err))
	}

	return "注册成功", registerRsp, constants.BizCodeSuccess
}

// DeleteUsers 删除用户
// 用户是否启用禁用需要实时更新contact_user_list状态，所以redis的contact_user_list需要删除
func (u *userInfoService) DeleteUsers(uuidList []string) (string, int) {
	if res := dao.GormDB.
		Where("uuid IN ?", uuidList).
		Delete(&model.UserInfo{}); res.Error != nil {
		zlog.Error("批量删除用户失败", zap.Error(res.Error))
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}

	if err := DelKeysByUUIDList("user_info", uuidList); err != nil {
		zlog.Warn("删除用户缓存失败", zap.Error(err))
	}
	if err := DelKeysByUUIDList("contact_mygroup_list", uuidList); err != nil {
		zlog.Warn("删除contact_user_list缓存失败", zap.Error(err))
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
			zlog.Debug("Redis 缓存未命中，回库读取", zap.String("key", cacheKey))
		} else {
			zlog.Warn("Redis 读取发生错误，回库读取", zap.Error(err), zap.String("key", cacheKey))
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
		if err := SetCache("user_info", rsp.Uuid, &rsp); err != nil {
			zlog.Warn("写入 Redis 缓存失败", zap.Error(err))
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
	var user model.UserInfo
	if res := dao.GormDB.First(&user, "uuid = ?", updateReq.Uuid); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			zlog.Debug("该用户不存在，修改失败")
			return "该用户不存在，修改失败", constants.BizCodeInvalid
		}
		zlog.Error(res.Error.Error())
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
	if res := dao.GormDB.Save(&user); res.Error != nil {
		zlog.Error(res.Error.Error())
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
	if err := SetCache("user_info", rsp.Uuid, &rsp); err != nil {
		zlog.Warn("写入 Redis 缓存失败", zap.Error(err))
	}

	return "修改用户信息成功", constants.BizCodeSuccess
}
