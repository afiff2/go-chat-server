package gorm

import (
	"testing"

	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserContactService end‑to‑end flow covering all public methods.
// The test re‑uses the GroupFlow sample you provided as a reference and
// focuses on the happy‑path plus key edge‑cases for every exported function
// inside user_contact_service.go.
func TestUserContactService(t *testing.T) {
	var ownerTel = "13800000111"
	var friendTel = "13800000112"

	var ownerId, friendId, groupId, group2Id string

	//--------------------------------------------------------------------
	// 0. Register two users that will participate in the whole scenario
	//--------------------------------------------------------------------
	t.Run("RegisterOwner", func(t *testing.T) {
		msg, rsp, code := UserInfoService.Register(request.RegisterRequest{
			Telephone: ownerTel, Password: "pass123", Nickname: "owner_user"})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "注册成功")
		ownerId = rsp.Uuid
	})

	t.Run("RegisterFriend", func(t *testing.T) {
		msg, rsp, code := UserInfoService.Register(request.RegisterRequest{
			Telephone: friendTel, Password: "pass123", Nickname: "friend_user"})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "注册成功")
		friendId = rsp.Uuid
	})

	//--------------------------------------------------------------------
	// 1. Friend 向 Owner 发起加好友申请
	//--------------------------------------------------------------------
	t.Run("ApplyContact", func(t *testing.T) {
		msg, code := UserContactService.ApplyContact(request.ApplyContactRequest{
			OwnerId: friendId, ContactId: ownerId, Message: "let's be friends"})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "申请成功")
	})

	//--------------------------------------------------------------------
	// 2. Owner 查看新的联系人申请列表
	//--------------------------------------------------------------------
	t.Run("GetNewContactList", func(t *testing.T) {
		msg, list, code := UserContactService.GetNewContactList(ownerId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取成功")
		require.Len(t, list, 1)
		assert.Equal(t, friendId, list[0].ContactId)
	})

	//--------------------------------------------------------------------
	// 3. Owner 同意申请 – PassContactApply (owner 是 U 开头)
	//--------------------------------------------------------------------
	t.Run("PassContactApply_User", func(t *testing.T) {
		msg, code := UserContactService.PassContactApply(ownerId, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "已添加该联系人")
	})

	//--------------------------------------------------------------------
	// 3.5. GetContactInfo (User)
	//--------------------------------------------------------------------
	t.Run("GetContactInfo_User", func(t *testing.T) {
		msg, info, code := UserContactService.GetContactInfo(friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取联系人信息成功")
		assert.Equal(t, friendId, info.ContactId)
		assert.Equal(t, "friend_user", info.ContactName)
	})

	//--------------------------------------------------------------------
	// 4. Owner 获取联系人列表，应包含 friend
	//--------------------------------------------------------------------
	t.Run("GetUserList", func(t *testing.T) {
		msg, list, code := UserContactService.GetUserList(ownerId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取用户列表成功")
		found := false
		for _, u := range list {
			if u.UserId == friendId {
				found = true
			}
		}
		assert.True(t, found, "联系人列表应包含 friend")
	})

	//--------------------------------------------------------------------
	// 5. Owner 创建群聊，配置为无需审核 (AddMode=0)
	//--------------------------------------------------------------------
	t.Run("CreateGroup", func(t *testing.T) {
		msg, code := GroupInfoService.CreateGroup(request.CreateGroupRequest{
			Name: "chat_group", OwnerId: ownerId, Notice: "welcome", AddMode: 0, Avatar: "avatar.png"})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "创建成功")

		_, myGroups, _ := GroupInfoService.LoadMyGroup(ownerId)
		require.Len(t, myGroups, 1)
		groupId = myGroups[0].GroupId
	})

	//--------------------------------------------------------------------
	// 6. friend 直接进群（无需审核）
	//--------------------------------------------------------------------
	t.Run("EnterGroupDirectly", func(t *testing.T) {
		msg, code := GroupInfoService.EnterGroupDirectly(groupId, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "进群成功")
	})

	//--------------------------------------------------------------------
	// 6.5. GetContactInfo (Group)
	//--------------------------------------------------------------------
	t.Run("GetContactInfo_Group", func(t *testing.T) {
		msg, info, code := UserContactService.GetContactInfo(groupId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取联系人信息成功")
		assert.Equal(t, groupId, info.ContactId)
		assert.Equal(t, "chat_group", info.ContactName)
		assert.Equal(t, 2, info.ContactMemberCnt)
	})

	//--------------------------------------------------------------------
	// 7. friend 查看自己已加入的群聊列表
	//--------------------------------------------------------------------
	t.Run("LoadMyJoinedGroup", func(t *testing.T) {
		msg, list, code := UserContactService.LoadMyJoinedGroup(friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取加入群成功")
		require.Len(t, list, 1)
		assert.Equal(t, groupId, list[0].GroupId)
	})

	//--------------------------------------------------------------------
	// 8. Owner 拉黑 friend
	//--------------------------------------------------------------------
	t.Run("BlackContact", func(t *testing.T) {
		msg, code := UserContactService.BlackContact(ownerId, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "已拉黑")
	})

	//--------------------------------------------------------------------
	// 9. 取消拉黑
	//--------------------------------------------------------------------
	t.Run("CancelBlackContact", func(t *testing.T) {
		msg, code := UserContactService.CancelBlackContact(ownerId, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "已解除拉黑")
	})

	//--------------------------------------------------------------------
	// 10. 删除好友
	//--------------------------------------------------------------------
	t.Run("DeleteContact", func(t *testing.T) {
		msg, code := UserContactService.DeleteContact(ownerId, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除联系人成功")
	})

	//--------------------------------------------------------------------
	// 11. 创建新群，模拟加群申请流程 (AddMode=1 审核制)
	//--------------------------------------------------------------------
	t.Run("CreateGroupNeedVerify", func(t *testing.T) {
		msg, code := GroupInfoService.CreateGroup(request.CreateGroupRequest{
			Name: "verify_group", OwnerId: ownerId, Notice: "need verify", AddMode: 1, Avatar: "v.png"})
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "创建成功")
		_, list, _ := GroupInfoService.LoadMyGroup(ownerId)
		for _, g := range list {
			if g.GroupName == "verify_group" {
				group2Id = g.GroupId //覆盖为需要审核的群
			}
		}
	})

	t.Run("ApplyContact_Group", func(t *testing.T) {
		msg, code := UserContactService.ApplyContact(request.ApplyContactRequest{
			OwnerId: friendId, ContactId: group2Id, Message: "let me in"})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "申请成功")
	})

	t.Run("GetAddGroupList", func(t *testing.T) {
		msg, list, code := UserContactService.GetAddGroupList(group2Id)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取成功")
		require.Len(t, list, 1)
		assert.Equal(t, friendId, list[0].ContactId)
	})

	//--------------------------------------------------------------------
	// 12. 群主拒绝申请
	//--------------------------------------------------------------------
	t.Run("RefuseContactApply_Group", func(t *testing.T) {
		msg, code := UserContactService.RefuseContactApply(group2Id, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "已拒绝")

		// 再次查询应没有其他申请
		_, listAfter, _ := UserContactService.GetAddGroupList(group2Id)
		assert.Len(t, listAfter, 0)
	})

	//--------------------------------------------------------------------
	// 13. 拉黑申请记录
	//--------------------------------------------------------------------
	t.Run("BlackApply", func(t *testing.T) {
		// 重新提交一次申请，随后拉黑它
		_, _ = UserContactService.ApplyContact(request.ApplyContactRequest{
			OwnerId: friendId, ContactId: group2Id, Message: "again"})
		msg, code := UserContactService.BlackApply(group2Id, friendId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "已拉黑")
	})

	//--------------------------------------------------------------------
	// 14. 删除群聊
	//--------------------------------------------------------------------
	t.Run("DisableEnableGroup", func(t *testing.T) {
		msg, code := GroupInfoService.DismissGroup(ownerId, groupId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "解散群聊成功")
		msg, code = GroupInfoService.DismissGroup(ownerId, group2Id)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "解散群聊成功")
	})

	//--------------------------------------------------------------------
	// 15. 清理资源，删除用户
	//--------------------------------------------------------------------
	t.Run("CleanupUsers", func(t *testing.T) {
		msg, code := UserInfoService.DeleteUsers([]string{ownerId, friendId})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除用户成功")
	})
}
