package gorm

import (
	"testing"

	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionService(t *testing.T) {
	var ownerTel = "13800000211"
	var friendTel = "13800000212"

	var ownerId, friendId, groupId string
	var sessionId, groupSessionId string

	//----------------------------------------------------------------
	// 0. 注册两名用户
	//----------------------------------------------------------------
	t.Run("RegisterOwner", func(t *testing.T) {
		msg, rsp, code := UserInfoService.Register(request.RegisterRequest{
			Telephone: ownerTel, Password: "pass123", Nickname: "session_owner"})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "注册成功")
		ownerId = rsp.Uuid
	})

	t.Run("RegisterFriend", func(t *testing.T) {
		msg, rsp, code := UserInfoService.Register(request.RegisterRequest{
			Telephone: friendTel, Password: "pass123", Nickname: "session_friend"})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "注册成功")
		friendId = rsp.Uuid
	})

	//----------------------------------------------------------------
	// 1. CreateSession 负面测试用例
	//----------------------------------------------------------------
	t.Run("CreateSession_EmptyReceive", func(t *testing.T) {
		msg, _, code := SessionService.CreateSession(request.OpenSessionRequest{
			SendId: ownerId, ReceiveId: ""})
		assert.Equal(t, constants.BizCodeInvalid, code)
		assert.Contains(t, msg, "目标ID不能为空")
	})

	t.Run("CreateSession_Self", func(t *testing.T) {
		msg, _, code := SessionService.CreateSession(request.OpenSessionRequest{
			SendId: ownerId, ReceiveId: ownerId})
		assert.Equal(t, constants.BizCodeInvalid, code)
		assert.Contains(t, msg, "不能自己和自己")
	})

	//----------------------------------------------------------------
	// 2. CheckOpenSessionAllowed - 在未添加联系人前应失败
	//----------------------------------------------------------------
	t.Run("CheckOpenSessionAllowed_NoContact", func(t *testing.T) {
		msg, ok, code := SessionService.CheckOpenSessionAllowed(ownerId, friendId)
		assert.Equal(t, constants.BizCodeInvalid, code)
		assert.False(t, ok)
		assert.Contains(t, msg, "未添加联系人")
	})

	//----------------------------------------------------------------
	// 3. 建立联系人关系
	//----------------------------------------------------------------
	t.Run("AddAsContacts", func(t *testing.T) {
		// 好友申请
		_, code := UserContactService.ApplyContact(request.ApplyContactRequest{
			OwnerId: friendId, ContactId: ownerId, Message: "hi"})
		require.Equal(t, constants.BizCodeSuccess, code)
		// 主用户同意申请
		_, code = UserContactService.PassContactApply(ownerId, friendId)
		require.Equal(t, constants.BizCodeSuccess, code)
	})

	t.Run("CheckOpenSessionAllowed_AllowedNow", func(t *testing.T) {
		msg, ok, code := SessionService.CheckOpenSessionAllowed(ownerId, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.True(t, ok)
		assert.Contains(t, msg, "可以发起会话")
	})

	//----------------------------------------------------------------
	// 4. 正常流程创建会话 (用户对用户)
	//----------------------------------------------------------------
	t.Run("CreateSession_User", func(t *testing.T) {
		msg, sid, code := SessionService.CreateSession(request.OpenSessionRequest{
			SendId: ownerId, ReceiveId: friendId})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "会话创建成功")
		require.NotEmpty(t, sid)
		sessionId = sid
	})

	//----------------------------------------------------------------
	// 5. 再次打开会话 – 应该复用同一个 session id（缓存命中）
	//----------------------------------------------------------------
	t.Run("OpenSession_Hit", func(t *testing.T) {
		msg, sid, code := SessionService.OpenSession(request.OpenSessionRequest{
			SendId: ownerId, ReceiveId: friendId})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "会话创建成功")
		assert.Equal(t, sessionId, sid, "OpenSession should return identical sessionId")
	})

	//----------------------------------------------------------------
	//  6. GetUserSessionList 应包含好友会话
	//----------------------------------------------------------------
	t.Run("GetUserSessionList", func(t *testing.T) {
		msg, list, code := SessionService.GetUserSessionList(ownerId)
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "获取成功")
		found := false
		for _, s := range list {
			if s.SessionId == sessionId && s.UserId == friendId {
				found = true
			}
		}
		assert.True(t, found, "用户会话列表应包含刚才创建的会话")
	})

	//----------------------------------------------------------------
	// 7. 群组会话流程
	//----------------------------------------------------------------
	t.Run("CreateGroup", func(t *testing.T) {
		msg, code := GroupInfoService.CreateGroup(request.CreateGroupRequest{
			Name: "session_group", OwnerId: ownerId, Notice: "grp", AddMode: 0, Avatar: "g.png"})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "创建成功")
		_, groups, _ := GroupInfoService.LoadMyGroup(ownerId)
		require.Len(t, groups, 1)
		groupId = groups[0].GroupId
	})

	t.Run("EnterGroupDirectly", func(t *testing.T) {
		msg, code := GroupInfoService.EnterGroupDirectly(groupId, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "进群成功")
	})

	t.Run("CreateSession_Group", func(t *testing.T) {
		msg, sid, code := SessionService.CreateSession(request.OpenSessionRequest{
			SendId: ownerId, ReceiveId: groupId})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "会话创建成功")
		groupSessionId = sid
	})

	t.Run("GetGroupSessionList", func(t *testing.T) {
		msg, list, code := SessionService.GetGroupSessionList(ownerId)
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "获取成功")
		found := false
		for _, s := range list {
			if s.SessionId == groupSessionId && s.GroupId == groupId {
				found = true
			}
		}
		assert.True(t, found, "群聊会话列表应包含刚才创建的会话")
	})

	//----------------------------------------------------------------
	// 8.  8. 删除会话 – 先删除用户会话，再删除群组会话
	//----------------------------------------------------------------
	t.Run("DeleteSession_User", func(t *testing.T) {
		msg, code := SessionService.DeleteSession(ownerId, friendId, sessionId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除成功")

		// 再次查询列表 – 应为空
		_, list, _ := SessionService.GetUserSessionList(ownerId)
		assert.Len(t, list, 0)
	})

	t.Run("DeleteSession_Group", func(t *testing.T) {
		msg, code := SessionService.DeleteSession(ownerId, groupId, groupSessionId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除成功")

		// 再次查询列表 – 应为空
		_, list, _ := SessionService.GetGroupSessionList(ownerId)
		assert.Len(t, list, 0)
	})

	//----------------------------------------------------------------
	// 9. 清理资源
	//----------------------------------------------------------------
	t.Run("Cleanup", func(t *testing.T) {
		_, code := GroupInfoService.DismissGroup(ownerId, groupId)
		assert.Equal(t, constants.BizCodeSuccess, code)

		msg, code := UserInfoService.DeleteUsers([]string{ownerId, friendId})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除用户成功")
	})
}
