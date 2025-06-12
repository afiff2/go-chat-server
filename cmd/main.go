package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/afiff2/go-chat-server/internal/https_server"
	"github.com/afiff2/go-chat-server/pkg/zlog"
)

func main() {

	go func() {
		if err := https_server.GE.Run(":8080"); err != nil {
			zlog.Fatal("server running fault")
			return
		}
	}()

	// 设置信号监听
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-quit

}
