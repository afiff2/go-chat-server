package https_server

import (
	v1 "github.com/afiff2/go-chat-server/api"
	"github.com/afiff2/go-chat-server/internal/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var GinEngine *gin.Engine

func init() {
	GinEngine = gin.Default()

	GinEngine.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders: []string{"Content-Length"},
	}))

	GinEngine.Static("/static/avatars", config.GetConfig().StaticSrc.StaticAvatarPath) // 映射头像目录
	GinEngine.Static("/static/files", config.GetConfig().StaticSrc.StaticFilePath)

	userGroup := GinEngine.Group("/user")
	{
		userGroup.POST("/register", v1.Register)     // 注册
		userGroup.POST("/login", v1.Login)           // 登录
		userGroup.POST("/delete", v1.DeleteUsers)    // 删除用户
		userGroup.POST("/get", v1.GetUserInfo)       // 获取用户信息
		userGroup.POST("/update", v1.UpdateUserInfo) // 更新用户信息
		userGroup.POST("/list", v1.GetUserInfoList)  // 获取用户列表
		userGroup.POST("/enable", v1.AbleUsers)      // 启用用户
		userGroup.POST("/disable", v1.DisableUsers)  // 禁用用户
		userGroup.POST("/set-admin", v1.SetAdmin)    // 设置管理员
	}

	// 群聊相关 API 路由
	groupGroup := GinEngine.Group("/group")
	{
		groupGroup.POST("/create", v1.CreateGroup)                // 创建群聊
		groupGroup.POST("/load-my", v1.LoadMyGroup)               // 获取我创建的群聊
		groupGroup.POST("/load-joined", v1.LoadMyJoinedGroup)     // 获取我加入的群聊
		groupGroup.POST("/check-add-mode", v1.CheckGroupAddMode)  // 检查群聊加群方式
		groupGroup.POST("/enter", v1.EnterGroupDirectly)          // 直接进群
		groupGroup.POST("/leave", v1.LeaveGroup)                  // 退群
		groupGroup.POST("/dismiss", v1.DismissGroup)              // 解散群聊
		groupGroup.POST("/info", v1.GetGroupInfo)                 // 获取群聊详情
		groupGroup.POST("/info-list", v1.GetGroupInfoList)        // 获取群聊列表（管理员）
		groupGroup.POST("/delete", v1.DeleteGroups)               // 删除群聊（管理员）
		groupGroup.POST("/set-status", v1.SetGroupsStatus)        // 设置群聊是否启用
		groupGroup.POST("/update", v1.UpdateGroupInfo)            // 更新群聊信息
		groupGroup.POST("/members", v1.GetGroupMemberList)        // 获取群聊成员列表
		groupGroup.POST("/remove-members", v1.RemoveGroupMembers) // 移除群聊成员
	}

	// 聊天记录相关 API 路由
	messageGroup := GinEngine.Group("/message")
	{
		messageGroup.POST("/list", v1.GetMessageList)            // 获取聊天记录
		messageGroup.POST("/group-list", v1.GetGroupMessageList) // 获取群聊消息记录
		messageGroup.POST("/upload-avatar", v1.UploadAvatar)     // 上传头像
		messageGroup.POST("/upload-file", v1.UploadFile)         // 上传文件
	}

	// 会话相关 API 路由
	sessionGroup := GinEngine.Group("/session")
	{
		sessionGroup.POST("/open", v1.OpenSession)                      // 打开会话
		sessionGroup.POST("/user-list", v1.GetUserSessionList)          // 获取用户会话列表
		sessionGroup.POST("/group-list", v1.GetGroupSessionList)        // 获取群聊会话列表
		sessionGroup.POST("/delete", v1.DeleteSession)                  // 删除会话
		sessionGroup.POST("/check-allowed", v1.CheckOpenSessionAllowed) // 检查是否可以打开会话
	}

	// 联系人相关 API 路由
	contactGroup := GinEngine.Group("/contact")
	{
		contactGroup.POST("/list", v1.GetUserList)                // 获取联系人列表
		contactGroup.POST("/info", v1.GetContactInfo)             // 获取联系人信息
		contactGroup.POST("/delete", v1.DeleteContact)            // 删除联系人
		contactGroup.POST("/apply", v1.ApplyContact)              // 申请添加联系人
		contactGroup.POST("/new-list", v1.GetNewContactList)      // 获取新的联系人申请列表
		contactGroup.POST("/pass-apply", v1.PassContactApply)     // 通过联系人申请
		contactGroup.POST("/refuse-apply", v1.RefuseContactApply) // 拒绝联系人申请
		contactGroup.POST("/black", v1.BlackContact)              // 拉黑联系人
		contactGroup.POST("/cancel-black", v1.CancelBlackContact) // 解除拉黑联系人
		contactGroup.POST("/add-group-list", v1.GetAddGroupList)  // 获取新的群聊申请列表
		contactGroup.POST("/black-apply", v1.BlackApply)          // 拉黑申请
	}

	// WebSocket 相关 API 路由
	wsGroup := GinEngine.Group("/ws")
	{
		wsGroup.GET("/login", v1.WsLogin)    // WebSocket 登录
		wsGroup.POST("/logout", v1.WsLogout) // WebSocket 登出
	}

}
