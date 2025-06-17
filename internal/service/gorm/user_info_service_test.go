package gorm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/dto/request"
	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/enum/user_info/user_status_enum"
)

func TestMain(m *testing.M) {

	code := m.Run()

	myredis.Close()
	dao.CloseDB()

	os.Exit(code)
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
	})

	testTel2 := "13800000002"
	var uuid2 string
	t.Run("Register2", func(t *testing.T) {
		req := request.RegisterRequest{
			Telephone: testTel2,
			Password:  "password123",
			Nickname:  "flow_user2",
		}
		_, rsp, code := UserInfoService.Register(req)
		require.Equal(t, constants.BizCodeSuccess, code)
		uuid2 = rsp.Uuid
	})

	t.Run("DisableUsers", func(t *testing.T) {
		// 禁用用户
		msg, code := UserInfoService.DisableUsers([]string{uuid2})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "禁用用户成功")

		// 再 Get，一定会回库 —— 因为缓存被删了
		msg2, _, code2 := UserInfoService.GetUserInfo(uuid2)
		assert.Equal(t, constants.BizCodeSuccess, code2)
		assert.Contains(t, msg2, "获取用户信息成功")
	})

	t.Run("AbleUsers", func(t *testing.T) {
		// 启用用户
		msg, code := UserInfoService.AbleUsers([]string{uuid2})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "启用用户成功")

		// 确保状态 NORMAL
		_, info, _ := UserInfoService.GetUserInfo(uuid2)
		assert.Equal(t, (int8)(user_status_enum.NORMAL), info.Status)
	})

	t.Run("SetAdmin", func(t *testing.T) {
		// 设置管理员状态
		msg, code := UserInfoService.SetAdmin([]string{uuid2}, 1)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "设置管理员成功")

		// 再拉一次，检查 IsAdmin 字段
		_, info, _ := UserInfoService.GetUserInfo(uuid2)
		assert.Equal(t, int8(1), info.IsAdmin)
	})

	testTel3 := "13800000003"
	var uuid3 string

	t.Run("GetUserInfoList", func(t *testing.T) {
		req := request.RegisterRequest{
			Telephone: testTel3,
			Password:  "password321",
			Nickname:  "flow_user3",
		}
		_, rsp, code := UserInfoService.Register(req)
		require.Equal(t, constants.BizCodeSuccess, code)
		uuid3 = rsp.Uuid

		// 列表里应该能看到刚才注册的两个用户（除了自己）
		msg, list, code := UserInfoService.GetUserInfoList(uuid2)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取用户列表成功")

		// var 一次声明多个 bool
		var found1, found2, found3 bool
		for _, u := range list {
			if u.Uuid == uuid {
				found1 = true
			}
			if u.Uuid == uuid2 {
				found2 = true
			}
			if u.Uuid == uuid3 {
				found3 = true
			}
		}
		assert.True(t, found1, "列表中应该包含 uuid1")
		assert.False(t, found2, "列表中不应该包含 uuid2")
		assert.True(t, found3, "列表中应该包含 uuid3")

		msg, code = UserInfoService.DeleteUsers([]string{uuid, uuid2, uuid3})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除用户成功")
	})
}
