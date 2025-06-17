package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/afiff2/go-chat-server/internal/https_server"
	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/zlog"
)

func main() {

	go func() {
		if err := https_server.GinEngine.Run(":8080"); err != nil {
			zlog.Fatal("server running fault")
			return
		}
	}()

	// 设置信号监听
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-quit

	// 删除所有Redis键
	if err := myredis.DeleteAllRedisKeys(); err != nil {
		zlog.Error(err.Error())
	} else {
		zlog.Info("所有Redis键已删除")
	}
}
