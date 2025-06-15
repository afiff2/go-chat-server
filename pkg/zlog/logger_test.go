package zlog

import (
	"testing"

	"go.uber.org/zap"
)

func TestZLog(t *testing.T) {

	Debug("这是一个调试日志", zap.String("category", "test"))
	Info("这是一个信息日志", zap.String("category", "test"))
	Warn("这是一个警告日志", zap.String("category", "test"))
	Error("这是一个错误日志", zap.String("category", "test"))
}
