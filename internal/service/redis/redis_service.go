package redis

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/afiff2/go-chat-server/internal/config"
	"github.com/afiff2/go-chat-server/pkg/zlog"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

var redisClient *redis.Client
var ctx = context.Background()

func init() {
	conf := config.GetConfig()
	host := conf.Redis.Host
	port := conf.Redis.Port
	password := conf.Redis.Password
	db := conf.Redis.Db
	addr := host + ":" + strconv.Itoa(port)

	redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		zlog.Fatal("无法连接到 Redis", zap.Error(err))
	}
}

func SetKeyEx(key string, value string, timeout time.Duration) error {
	err := redisClient.Set(ctx, key, value, timeout).Err()
	if err != nil {
		return err
	}
	return nil
}

func GetKeyNilIsErr(key string) (string, error) {
	value, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return value, nil
}

func scanKeys(pattern string) ([]string, error) {
	var cursor uint64
	var all []string
	for {
		keys, next, err := redisClient.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		all = append(all, keys...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return all, nil
}

func GetKeyWithPrefixNilIsErr(prefix string) (string, error) {
	var keys []string
	var err error

	// 使用 Keys 命令迭代匹配的键
	keys, err = scanKeys(prefix + "*")
	if err != nil {
		return "", err
	}

	if len(keys) == 0 {
		zlog.Debug("没有找到相关前缀key")
		return "", redis.Nil
	}

	if len(keys) == 1 {
		zlog.Debug("成功找到了相关前缀 key", zap.Strings("keys", keys))
		return keys[0], nil
	} else {
		zlog.Error("找到了数量大于1的key，查找异常")
		return "", errors.New("找到了数量大于1的key，查找异常")
	}
}

func GetKeyWithSuffixNilIsErr(suffix string) (string, error) {
	var keys []string
	var err error

	// 使用 Keys 命令迭代匹配的键
	keys, err = scanKeys("*" + suffix)
	if err != nil {
		return "", err
	}

	if len(keys) == 0 {
		zlog.Debug("没有找到相关后缀key")
		return "", redis.Nil
	}

	if len(keys) == 1 {
		zlog.Debug("成功找到了相关后缀key", zap.Strings("keys", keys))
		return keys[0], nil
	} else {
		zlog.Error("找到了数量大于1的key，查找异常")
		return "", errors.New("找到了数量大于1的key，查找异常")
	}

}

func DelKeyIfExists(key string) error {
	delErr := redisClient.Del(ctx, key).Err()
	if delErr != nil {
		return delErr
	}
	return nil
}

// deleteByPatternBatch 按批次 SCAN + DEL，避免一次性把所有 key 都加载到内存
func deleteByPatternBatch(pattern string) error {
	var cursor uint64
	for {
		// 每次 SCAN 最多取 100 条
		keys, nextCursor, err := redisClient.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		// 如果这一批有 key，就立刻删
		if len(keys) > 0 {
			if _, err := redisClient.Del(ctx, keys...).Result(); err != nil {
				return err
			}
			zlog.Debug("成功删除一批 Redis key", zap.Int("count", len(keys)), zap.String("pattern", pattern))
		}

		// 游标归零说明扫描完毕
		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}
	return nil
}

func DelKeysWithPrefix(prefix string) error {
	return deleteByPatternBatch(prefix + "*")
}
func DelKeysWithSuffix(suffix string) error {
	return deleteByPatternBatch("*" + suffix)
}

func DeleteAllRedisKeys() error {
	return deleteByPatternBatch("*")
}

// DelKeys 批量删除给定的一组 key，底层使用 pipeline，
func DelKeys(keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	// 开启 pipeline
	pipe := redisClient.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}

	// 一次性执行所有命令
	if _, err := pipe.Exec(ctx); err != nil {
		zlog.Warn("Redis 批量删除缓存失败", zap.Error(err), zap.Strings("keys", keys))
		return err
	}
	return nil
}

// SetStringKeys 批量设置 Redis key-value（已序列化字符串），使用 pipeline
func SetStringKeys(data map[string]string, expiration time.Duration) error {
	if len(data) == 0 {
		return nil
	}

	pipe := redisClient.Pipeline()
	for key, val := range data {
		pipe.Set(ctx, key, val, expiration)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		zlog.Warn("Redis 批量写入失败", zap.Error(err))
		return err
	}
	return nil
}
