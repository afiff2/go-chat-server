package dao

import (
	"os"
	"testing"
	"time"

	"github.com/afiff2/go-chat-server/internal/model"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	// 如果需要的话，这里可以做一些初始化，比如清空 Redis、重建 DB 表等

	code := m.Run() // 先跑所有 TestXXX

	CloseDB()

	os.Exit(code)
}

func TestCreateUser(t *testing.T) {
	zlog.Info("开始执行 TestCreateUser")
	// 假设 GormDB 已经在 init 中初始化好了
	db := GormDB

	user := &model.UserInfo{
		Uuid:      "testuser123",
		Nickname:  "Test User",
		Telephone: "12345678901",
		Password:  "password123",
		CreatedAt: time.Now(),
	}

	zlog.Info("正在创建用户", zap.Any("user", user))
	result := db.Create(user)
	assert.NoError(t, result.Error)
	assert.NotZero(t, user.Id)

	zlog.Info("正在查询用户", zap.String("uuid", user.Uuid))
	var foundUser model.UserInfo
	err := db.Where("uuid = ?", user.Uuid).First(&foundUser).Error
	assert.NoError(t, err)
	assert.Equal(t, user.Uuid, foundUser.Uuid)
	assert.Equal(t, user.Nickname, foundUser.Nickname)

	// 清理数据（硬）
	zlog.Warn("正在删除测试用户", zap.String("uuid", user.Uuid))
	db.Unscoped().Delete(user)
}
