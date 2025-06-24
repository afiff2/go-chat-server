package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/afiff2/go-chat-server/internal/dao"
	"github.com/afiff2/go-chat-server/internal/https_server"
	"github.com/afiff2/go-chat-server/internal/service/chat"
	myKafka "github.com/afiff2/go-chat-server/internal/service/kafka"
	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"go.uber.org/zap"
)

func main() {
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		chat.KafkaChatServer.Start(rootCtx)
	}()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: https_server.GinEngine,
	}

	go func() {
		zlog.Info("HTTP 服务启动", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zlog.Fatal("HTTP 服务异常退出", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // 阻塞直到收到 SIGINT/SIGTERM

	zlog.Info("收到退出信号，开始关机")

	rootCancel()
	wg.Wait()
	myKafka.KafkaService.Close()

	//关闭 HTTP 服务，给 5 秒时间处理未完成请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zlog.Warn("HTTP 关机失败，强制关闭", zap.Error(err))
	} else {
		zlog.Info("HTTP 服务已关闭")
	}

	myredis.Close()
	dao.CloseDB()
	zlog.Info("所有资源已清理，程序退出")
}
