package v1

import (
	"net/http"

	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/internal/service/gorm"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/gin-gonic/gin"
)

// Register 注册
func Register(c *gin.Context) {
	var registerReq request.RegisterRequest
	if err := c.BindJSON(&registerReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, userInfo, ret := gorm.UserInfoService.Register(registerReq)
	SendResponse(c, message, ret, userInfo)
}

// Login 登录
func Login(c *gin.Context) {
	var loginReq request.LoginRequest
	if err := c.BindJSON(&loginReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, userInfo, ret := gorm.UserInfoService.Login(loginReq)
	SendResponse(c, message, ret, userInfo)
}

// DeleteUsers 删除用户
func DeleteUsers(c *gin.Context) {
	var req request.AbleUsersRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserInfoService.DeleteUsers(req.UuidList)
	SendResponse(c, message, ret, nil)
}

// GetUserInfo 获取用户信息
func GetUserInfo(c *gin.Context) {
	var req request.GetUserInfoRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, userInfo, ret := gorm.UserInfoService.GetUserInfo(req.Uuid)
	SendResponse(c, message, ret, userInfo)
}

// UpdateUserInfo 修改用户信息
func UpdateUserInfo(c *gin.Context) {
	var req request.UpdateUserInfoRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	message, ret := gorm.UserInfoService.UpdateUserInfo(req)
	SendResponse(c, message, ret, nil)
}

// // GetUserInfoList 获取用户列表
// func GetUserInfoList(c *gin.Context) {
// 	var req request.GetUserInfoListRequest
// 	if err := c.BindJSON(&req); err != nil {
// 		zlog.Error(err.Error())
// 		c.JSON(http.StatusOK, gin.H{
// 			"code":    500,
// 			"message": constants.SYSTEM_ERROR,
// 		})
// 		return
// 	}
// 	message, userList, ret := gorm.UserInfoService.GetUserInfoList(req.OwnerId)
// 	JsonBack(c, message, ret, userList)
// }

// // AbleUsers 启用用户
// func AbleUsers(c *gin.Context) {
// 	var req request.AbleUsersRequest
// 	if err := c.BindJSON(&req); err != nil {
// 		zlog.Error(err.Error())
// 		c.JSON(http.StatusOK, gin.H{
// 			"code":    500,
// 			"message": constants.SYSTEM_ERROR,
// 		})
// 		return
// 	}
// 	message, ret := gorm.UserInfoService.AbleUsers(req.UuidList)
// 	JsonBack(c, message, ret, nil)
// }

// // DisableUsers 禁用用户
// func DisableUsers(c *gin.Context) {
// 	var req request.AbleUsersRequest
// 	if err := c.BindJSON(&req); err != nil {
// 		zlog.Error(err.Error())
// 		c.JSON(http.StatusOK, gin.H{
// 			"code":    500,
// 			"message": constants.SYSTEM_ERROR,
// 		})
// 		return
// 	}
// 	message, ret := gorm.UserInfoService.DisableUsers(req.UuidList)
// 	JsonBack(c, message, ret, nil)
// }

// // SetAdmin 设置管理员
// func SetAdmin(c *gin.Context) {
// 	var req request.AbleUsersRequest
// 	if err := c.BindJSON(&req); err != nil {
// 		zlog.Error(err.Error())
// 		c.JSON(http.StatusOK, gin.H{
// 			"code":    500,
// 			"message": constants.SYSTEM_ERROR,
// 		})
// 		return
// 	}
// 	message, ret := gorm.UserInfoService.SetAdmin(req.UuidList, req.IsAdmin)
// 	JsonBack(c, message, ret, nil)
// }

// // SendSmsCode 发送短信验证码
// func SendSmsCode(c *gin.Context) {
// 	var req request.SendSmsCodeRequest
// 	if err := c.BindJSON(&req); err != nil {
// 		zlog.Error(err.Error())
// 		c.JSON(http.StatusOK, gin.H{
// 			"code":    500,
// 			"message": constants.SYSTEM_ERROR,
// 		})
// 		return
// 	}
// 	message, ret := gorm.UserInfoService.SendSmsCode(req.Telephone)
// 	JsonBack(c, message, ret, nil)
// }
