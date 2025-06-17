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

// data必须是指针
func SetCache[T any](prefix string, key string, data *T) error {
	cacheKey := fmt.Sprintf("%s_%s", prefix, key)

	rspBytes, err := json.Marshal(*data)
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

// DelKeysWithPrefix 批量删除给定前缀 + 一组 uuid 对应的 keys，底层使用 pipeline
func DelKeysByUUIDList(prefix string, uuids []string) error {
	if len(uuids) == 0 {
		return nil
	}

	// 拼出完整的 key 列表
	keys := make([]string, 0, len(uuids))
	for _, id := range uuids {
		keys = append(keys, prefix+"_"+id)
	}

	// 调用已有的批量删除
	return myredis.DelKeys(keys)
}
