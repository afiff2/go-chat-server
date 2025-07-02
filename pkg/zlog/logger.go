package zlog

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/afiff2/go-chat-server/internal/config"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func init() {
	cfg := config.GetConfig()

	err := InitLogger(cfg.Log.Path, cfg.Log.Level, cfg.Log.Env)
	if err != nil {
		panic("初始化日志系统失败: " + err.Error())
	}
}

// InitLogger 初始化日志系统
func InitLogger(logPath, levelStr, env string) error {
	if logPath == "" {
		return fmt.Errorf("log path is empty")
	}

	fileDir := filepath.Dir(logPath)
	if err := os.MkdirAll(fileDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("关闭临时日志文件失败: %w", err)
	}

	var encoder zapcore.Encoder
	if env == "dev" {
		encoder = zapcore.NewConsoleEncoder(getDevEncoderConfig())
	} else {
		encoder = zapcore.NewJSONEncoder(getProdEncoderConfig())
	}

	writeSyncer := getFileLogWriter(logPath)
	atomicLevel := getZapLevel(levelStr)

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), atomicLevel),
		zapcore.NewCore(encoder, writeSyncer, atomicLevel),
	)

	logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zap.ErrorLevel)) //跳过一层调用栈, 防止zlog/logger.go:132
	return nil
}

// 获取开发模式下的 Encoder 配置
func getDevEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.LevelKey = "level"
	cfg.NameKey = "logger"
	cfg.CallerKey = "caller"
	cfg.MessageKey = "message"
	cfg.StacktraceKey = "stacktrace"
	cfg.LineEnding = zapcore.DefaultLineEnding
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder // 带颜色的级别
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02 - 15:04:05")
	cfg.EncodeDuration = zapcore.StringDurationEncoder // 持续时间转字符串
	cfg.EncodeCaller = zapcore.ShortCallerEncoder      // 短路径文件名+行号
	return cfg
}

// getProdEncoderConfig 返回适用于生产环境的 Encoder 配置
func getProdEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder, // 小写级别：info/warn/error
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006/01/02 - 15:04:05"),
		EncodeDuration: zapcore.StringDurationEncoder, // 持续时间转字符串
		EncodeCaller:   zapcore.ShortCallerEncoder,    // 短路径文件名+行号
	}
}

func getZapLevel(levelStr string) zap.AtomicLevel {
	level := zap.InfoLevel
	switch levelStr {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	case "dpanic":
		level = zap.DPanicLevel
	case "panic":
		level = zap.PanicLevel
	case "fatal":
		level = zap.FatalLevel
	default:
		level = zap.InfoLevel
	}
	return zap.NewAtomicLevelAt(level)
}

func getFileLogWriter(filename string) zapcore.WriteSyncer {
	lj := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100,   // MB
		MaxBackups: 60,    // 最多保留60个备份
		MaxAge:     28,    // 28天一切割
		Compress:   false, // 不压缩旧日志
	}
	return zapcore.AddSync(lj)
}

func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	logger.Fatal(msg, fields...)
}
