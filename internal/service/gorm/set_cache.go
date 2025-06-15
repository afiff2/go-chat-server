package gorm

import (
	"encoding/json"
	"fmt"
	"time"

	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"go.uber.org/zap"
)

func SetCache[T any](prefix string, key string, data *T) error {
	cacheKey := fmt.Sprintf("%s_%s", prefix, key)

	rspBytes, err := json.Marshal(data)
	if err != nil {
		zlog.Error("数据序列化失败", zap.Error(err))
		return err
	}

	err = myredis.SetKeyEx(cacheKey, string(rspBytes), constants.REDIS_TIMEOUT*time.Minute)
	if err != nil {
		zlog.Error("写入 Redis 缓存失败", zap.Error(err))
		return err
	}
	return nil
}
