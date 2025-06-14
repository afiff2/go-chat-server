package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/internal/model"
	"github.com/afiff2/go-chat-server/pkg/constants"
)

// 初始化数据库和 Redis
func init() {
	// 假设 dao.GormDB 和 redis.Client 已经在 init 中初始化好了
	// 如果没有，请手动调用一次初始化函数
}

func TestUserFlow(t *testing.T) {
	// 生成唯一电话号码避免冲突
	testTel := "13800000001"

	t.Run("Register", func(t *testing.T) {
		req := request.RegisterRequest{
			Telephone: testTel,
			Password:  "password123",
			Nickname:  "test_user",
		}

		msg, _, code := UserInfoService.Register(req)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "注册成功")
	})

	t.Run("Login", func(t *testing.T) {
		req := request.LoginRequest{
			Telephone: testTel,
			Password:  "password123",
		}

		msg, userInfo, code := UserInfoService.Login(req)
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "登陆成功")
		require.NotNil(t, userInfo)

		assert.Equal(t, testTel, userInfo.Telephone)
		assert.Equal(t, "test_user", userInfo.Nickname)
	})

	t.Run("GetUserInfo", func(t *testing.T) {
		uuid := getTestUserUuid(t, testTel)

		msg, userInfo, code := UserInfoService.GetUserInfo(uuid)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取用户信息成功")

		assert.Equal(t, testTel, userInfo.Telephone)
		assert.Equal(t, "test_user", userInfo.Nickname)
	})

	t.Run("UpdateUserInfo", func(t *testing.T) {
		uuid := getTestUserUuid(t, testTel)

		req := request.UpdateUserInfoRequest{
			Uuid:      uuid,
			Nickname:  "updated_nick",
			Email:     "new@example.com",
			Birthday:  "19900101",
			Signature: "hello world",
		}

		msg, code := UserInfoService.UpdateUserInfo(req)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "修改用户信息成功")

		// 验证更新后的数据
		_, userInfo, _ := UserInfoService.GetUserInfo(uuid)
		assert.Equal(t, "updated_nick", userInfo.Nickname)
		assert.Equal(t, "new@example.com", userInfo.Email)
		assert.Equal(t, "19900101", userInfo.Birthday)
		assert.Equal(t, "hello world", userInfo.Signature)
	})

	t.Run("DeleteUsers", func(t *testing.T) {
		uuid := getTestUserUuid(t, testTel)

		msg, code := UserInfoService.DeleteUsers([]string{uuid})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除用户成功")

		// 确认无法再次获取
		_, _, code = UserInfoService.GetUserInfo(uuid)
		assert.Equal(t, constants.BizCodeInvalid, code)
	})
}

// 辅助函数：从数据库中获取用户的 UUID
func getTestUserUuid(t *testing.T, telephone string) string {
	var user model.UserInfo
	res := dao.GormDB.First(&user, "telephone = ?", telephone)
	require.NoError(t, res.Error)
	return user.Uuid
}
