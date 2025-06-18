package gorm

import (
	"testing"

	"github.com/afiff2/go-chat-server/internal/dto/request"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/enum/group_info/group_status_enum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupFlow(t *testing.T) {
	// 准备两个测试用户
	ownerTel := "13800000011"
	memberTel := "13800000012"
	var ownerId, memberId, groupId, group2Id string

	t.Run("RegisterOwner", func(t *testing.T) {
		req := request.RegisterRequest{Telephone: ownerTel, Password: "pass123", Nickname: "owner_user"}
		msg, rsp, code := UserInfoService.Register(req)
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "注册成功")
		ownerId = rsp.Uuid
	})

	t.Run("RegisterMember", func(t *testing.T) {
		req := request.RegisterRequest{Telephone: memberTel, Password: "pass123", Nickname: "member_user"}
		msg, rsp, code := UserInfoService.Register(req)
		require.Equal(t, constants.BizCodeSuccess, code)
		require.Contains(t, msg, "注册成功")
		memberId = rsp.Uuid
	})

	t.Run("CreateGroup", func(t *testing.T) {
		req := request.CreateGroupRequest{Name: "test_group", Notice: "notice", OwnerId: ownerId, AddMode: 0, Avatar: "avatar_url"}
		msg, code := GroupInfoService.CreateGroup(req)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "创建成功")
		_, list, _ := GroupInfoService.LoadMyGroup(ownerId)
		require.Len(t, list, 1)
		groupId = list[0].GroupId
	})

	t.Run("CreateGroup2", func(t *testing.T) {
		req := request.CreateGroupRequest{Name: "test_group2", Notice: "notice2", OwnerId: ownerId, AddMode: 1, Avatar: "avatar2"}
		msg, code := GroupInfoService.CreateGroup(req)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "创建成功")
		_, list, _ := GroupInfoService.LoadMyGroup(ownerId)
		require.True(t, len(list) >= 2)
		// 找到第二个群
		for _, g := range list {
			if g.GroupName == "test_group2" {
				group2Id = g.GroupId
			}
		}
		require.NotEmpty(t, group2Id)
	})

	t.Run("GetGroupInfoList", func(t *testing.T) {
		msg, list, code := GroupInfoService.GetGroupInfoList()
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "获取成功")
		var found bool
		for _, g := range list {
			if g.Uuid == groupId {
				found = true
			}
		}
		assert.True(t, found, "列表中应包含创建的群聊")
	})

	t.Run("DeleteGroupsByAdmin", func(t *testing.T) {
		msg, code := GroupInfoService.DeleteGroupsByAdmin([]string{group2Id})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "解散/删除群聊成功")
		_, _, code2 := GroupInfoService.GetGroupInfo(group2Id)
		assert.NotEqual(t, constants.BizCodeSuccess, code2)
	})

	t.Run("CheckGroupAddMode", func(t *testing.T) {
		msg, mode, code := GroupInfoService.CheckGroupAddMode(groupId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "加群方式获取成功")
		assert.Equal(t, int8(0), mode)
	})

	t.Run("SetGroupsStatus_Disable", func(t *testing.T) {
		msg, code := GroupInfoService.SetGroupsStatus([]string{groupId}, group_status_enum.DISABLE)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "设置成功")
		_, info, _ := GroupInfoService.GetGroupInfo(groupId)
		assert.Equal(t, (int8)(group_status_enum.DISABLE), info.Status)
	})

	t.Run("SetGroupsStatus_Enable", func(t *testing.T) {
		msg, code := GroupInfoService.SetGroupsStatus([]string{groupId}, group_status_enum.NORMAL)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "设置成功")
		_, info, _ := GroupInfoService.GetGroupInfo(groupId)
		assert.Equal(t, (int8)(group_status_enum.NORMAL), info.Status)
	})

	t.Run("UpdateGroupInfo", func(t *testing.T) {
		req := request.UpdateGroupInfoRequest{Uuid: groupId, Name: "new_name", Notice: "new_notice", AddMode: 1, Avatar: "new_avatar"}
		msg, code := GroupInfoService.UpdateGroupInfo(req)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "更新成功")
		_, info, _ := GroupInfoService.GetGroupInfo(groupId)
		assert.Equal(t, "new_name", info.Name)
		assert.Equal(t, "new_notice", info.Notice)
		assert.Equal(t, int8(1), info.AddMode)
		assert.Equal(t, "new_avatar", info.Avatar)
	})

	t.Run("EnterGroupDirectly", func(t *testing.T) {
		msg, code := GroupInfoService.EnterGroupDirectly(groupId, memberId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "进群成功")
	})

	t.Run("RemoveGroupMembers_CannotRemoveOwner", func(t *testing.T) {
		req := request.RemoveGroupMembersRequest{GroupId: groupId, OwnerId: ownerId, UuidList: []string{ownerId}}
		msg, code := GroupInfoService.RemoveGroupMembers(req)
		assert.Equal(t, constants.BizCodeInvalid, code)
		assert.Contains(t, msg, "不能移除群主")
	})

	t.Run("RemoveGroupMembers_Success", func(t *testing.T) {
		req := request.RemoveGroupMembersRequest{GroupId: groupId, OwnerId: ownerId, UuidList: []string{memberId}}
		msg, code := GroupInfoService.RemoveGroupMembers(req)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "移除群聊成员成功")
		_, members, _ := GroupInfoService.GetGroupMemberList(groupId)
		assert.Len(t, members, 1)
	})

	t.Run("LeaveGroup_OwnerCannotLeave", func(t *testing.T) {
		msg, code := GroupInfoService.LeaveGroup(ownerId, groupId)
		assert.Equal(t, constants.BizCodeInvalid, code)
		assert.Contains(t, msg, "群主不允许主动退出群聊")
	})

	t.Run("DismissGroup", func(t *testing.T) {
		msg, code := GroupInfoService.DismissGroup(ownerId, groupId)
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "解散群聊成功")
	})

	t.Run("GetAfterDismiss", func(t *testing.T) {
		_, _, code := GroupInfoService.GetGroupInfo(groupId)
		assert.NotEqual(t, constants.BizCodeSuccess, code)
	})

	t.Run("CleanupUsers", func(t *testing.T) {
		msg, code := UserInfoService.DeleteUsers([]string{ownerId, memberId})
		assert.Equal(t, constants.BizCodeSuccess, code)
		assert.Contains(t, msg, "删除用户成功")
	})
}
