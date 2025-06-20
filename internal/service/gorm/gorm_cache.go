package gorm

import (
	"encoding/json"
	"time"

	myredis "github.com/afiff2/go-chat-server/internal/service/redis"
	"github.com/afiff2/go-chat-server/pkg/constants"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"go.uber.org/zap"
)

// data必须是指针
func SetCache[T any](key string, data *T) error {
	rspBytes, err := json.Marshal(*data)
	if err != nil {
		zlog.Error("数据序列化失败", zap.Error(err))
		return err
	}

	err = myredis.SetKeyEx(key, string(rspBytes), constants.REDIS_TIMEOUT*time.Minute)
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

// DelKeysByPatternAndUUIDList 批量删除给定带通配符前缀 + 一组 uuid 对应的 keys。
// 例如，调用 DelKeysByPatternAndUUIDList("session_*", []string{"a","b","c"})，
// 会依次删除匹配 "session_*_a"、"session_*_b"、"session_*_c" 的所有 key。
func DelKeysByPatternAndUUIDList(prefixPattern string, uuids []string) error {
	if len(uuids) == 0 {
		return nil
	}

	for _, id := range uuids {
		// 构造完整的模式：prefixPattern + uuid
		// 例如 "session_*_" + "abc123" => "session_*_abc123"
		pattern := prefixPattern + "_" + id
		if err := myredis.DelKeysWithPattern(pattern); err != nil {
			return err
		}
	}

	return nil
}
