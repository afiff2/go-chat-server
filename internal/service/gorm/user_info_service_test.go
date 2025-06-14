package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/afiff2/go-chat-server/internal/dto/request"
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
	var uuid string

	t.Run("Register", func(t *testing.T) {
		req := request.RegisterRequest{
			Telephone: testTel,
			Password:  "password123",
			Nickname:  "test_user",
		}

		msg, rsp, code := UserInfoService.Register(req)
		uuid = rsp.Uuid
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
		msg, userInfo, code := UserInfoService.GetUserInfo(uuid)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取用户信息成功")

		assert.Equal(t, testTel, userInfo.Telephone)
		assert.Equal(t, "test_user", userInfo.Nickname)
	})

	t.Run("UpdateUserInfo", func(t *testing.T) {
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
		msg, code := UserInfoService.DeleteUsers([]string{uuid})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除用户成功")

		// 确认无法再次获取
		_, _, code = UserInfoService.GetUserInfo(uuid)
		assert.Equal(t, constants.BizCodeInvalid, code)
	})

	// 软删除后重新注册
	t.Run("RegisterAfterSoftDelete", func(t *testing.T) {
		req := request.RegisterRequest{
			Telephone: testTel,
			Password:  "newPassword",
			Nickname:  "test_user_again",
		}

		msg, _, code := UserInfoService.Register(req)
		assert.Contains(t, msg, "注册成功（恢复历史账号）")
		assert.Equal(t, constants.BizCodeSuccess, code)

		_, userInfo, _ := UserInfoService.GetUserInfo(uuid)
		assert.Equal(t, "test_user_again", userInfo.Nickname)

		msg2, code2 := UserInfoService.DeleteUsers([]string{uuid})
		assert.Equal(t, constants.BizCodeSuccess, code2)
		assert.Contains(t, msg2, "删除用户成功")
	})
}
