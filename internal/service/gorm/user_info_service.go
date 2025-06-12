package gorm

import (
	"errors"
	"fmt"
	"time"

	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/internal/dto/respond"
	"github.com/afiff2/go-chat-server/internal/model"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/enum/user_status_enum"
	myhash "github.com/afiff2/go-chat-server/pkg/util/hash"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"gorm.io/gorm"
)

type userInfoService struct {
}

var UserInfoService = new(userInfoService)

// Login 登录
func (u *userInfoService) Login(loginReq request.LoginRequest) (string, *respond.LoginRespond, int) {
	password := loginReq.Password
	var user model.UserInfo
	if res := dao.GormDB.First(&user, "telephone = ?", loginReq.Telephone); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			message := "用户不存在，请注册"
			zlog.Error(message)
			return message, nil, constants.BizCodeInvalid
		}
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	// 使用哈希验证密码
	if !myhash.CheckPasswordHash(password, user.Password) {
		message := "密码不正确，请重试"
		zlog.Error(message)
		return message, nil, constants.BizCodeInvalid
	}

	loginRsp := &respond.LoginRespond{
		Uuid:      user.Uuid,
		Telephone: user.Telephone,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Avatar:    user.Avatar,
		Gender:    user.Gender,
		Birthday:  user.Birthday,
		Signature: user.Signature,
		IsAdmin:   user.IsAdmin,
		Status:    user.Status,
	}
	year, month, day := user.CreatedAt.Date()
	loginRsp.CreatedAt = fmt.Sprintf("%d.%d.%d", year, month, day)

	return "登陆成功", loginRsp, constants.BizCodeSuccess
}

// Register 注册，返回(message, register_respond_string, error)
func (u *userInfoService) Register(registerReq request.RegisterRequest) (string, *respond.RegisterRespond, int) {

	// 不用校验手机号，前端校验
	// 判断电话是否已经被注册过了
	var user model.UserInfo
	if res := dao.GormDB.First(&user, "telephone = ?", registerReq.Telephone); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			zlog.Info("该电话不存在，可以注册")
		} else {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, nil, constants.BizCodeError
		}
	} else {
		zlog.Info("该电话已经存在，注册失败")
		return "该电话已经存在，注册失败", nil, constants.BizCodeInvalid
	}

	// 加密密码
	hashedPassword, err := myhash.HashPassword(registerReq.Password)
	if err != nil {
		zlog.Error("密码加密失败", zap.Error(err))
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}

	var newUser model.UserInfo
	newUser.Uuid = uuid.NewString()
	newUser.Telephone = registerReq.Telephone
	newUser.Password = hashedPassword
	newUser.Nickname = registerReq.Nickname
	newUser.Avatar = "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png"
	newUser.CreatedAt = time.Now()
	newUser.IsAdmin = 0
	newUser.Status = user_status_enum.NORMAL
	// 手机号验证，最后一步才调用api，省钱hhh
	//err := sms.VerificationCode(registerReq.Telephone)
	//if err != nil {
	//	zlog.Error(err.Error())
	//	return "", err
	//}

	res := dao.GormDB.Create(&newUser)
	if res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, constants.BizCodeError
	}
	// 注册成功，chat client建立
	//if err := chat.NewClientInit(c, newUser.Uuid); err != nil {
	//	return "", err
	//}
	registerRsp := &respond.RegisterRespond{
		Uuid:      newUser.Uuid,
		Telephone: newUser.Telephone,
		Nickname:  newUser.Nickname,
		Email:     newUser.Email,
		Avatar:    newUser.Avatar,
		Gender:    newUser.Gender,
		Birthday:  newUser.Birthday,
		Signature: newUser.Signature,
		IsAdmin:   newUser.IsAdmin,
		Status:    newUser.Status,
	}
	year, month, day := newUser.CreatedAt.Date()
	registerRsp.CreatedAt = fmt.Sprintf("%d.%d.%d", year, month, day)

	return "注册成功", registerRsp, constants.BizCodeSuccess
}

// DeleteUsers 删除用户
// 用户是否启用禁用需要实时更新contact_user_list状态，所以redis的contact_user_list需要删除
func (u *userInfoService) DeleteUsers(uuidList []string) (string, int) {
	//zlog.Debug("计划删除用户数量", zap.Int("length", len(uuidList)))
	var users []model.UserInfo
	if res := dao.GormDB.Model(model.UserInfo{}).Where("uuid in (?)", uuidList).Find(&users); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, constants.BizCodeError
	}
	for _, user := range users {
		user.DeletedAt.Valid = true
		user.DeletedAt.Time = time.Now()
		if res := dao.GormDB.Save(&user); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, constants.BizCodeError
		}
	}
	//zlog.Debug("删除用户数量", zap.Int("length", len(users)))
	return "删除用户成功", constants.BizCodeSuccess
}
