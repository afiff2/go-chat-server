package dao

import (
	"fmt"
	"os"

	"github.com/afiff2/go-chat-server/internal/config"
	"github.com/afiff2/go-chat-server/internal/model"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"go.uber.org/zap"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var GormDB *gorm.DB

// 初始化
func init() {
	conf := config.GetConfig()
	user := conf.Mysql.User
	socket := conf.Mysql.Socket
	databaseName := conf.Mysql.DatabaseName

	dsn := fmt.Sprintf("%s@unix(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, socket, databaseName)

	var err error
	GormDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		zlog.Error("打开GormDB失败", zap.Error(err))
		os.Exit(1)
	}

	err = GormDB.AutoMigrate(&model.UserInfo{}, &model.GroupInfo{}, &model.UserContact{}, &model.Session{}, &model.ContactApply{}, &model.Message{})
	if err != nil {
		zlog.Error("GormDB自动迁移失败", zap.Error(err))
		os.Exit(1)
	}

	zlog.Info("数据库连接和自动迁移成功")
}

func CloseDB() {
	sqlDB, err := GormDB.DB()
	if err != nil {
		zlog.Error("获取底层 sql.DB 失败，无法关闭", zap.Error(err))
		return
	}
	if err := sqlDB.Close(); err != nil {
		zlog.Error("关闭数据库连接失败", zap.Error(err))
	} else {
		zlog.Info("数据库连接已关闭")
	}
}
