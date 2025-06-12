package redis

import (
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func TestDelKeyIfExists_Basic(t *testing.T) {
	// Case 1: Key 存在
	key := "test_del_exists"
	SetKeyEx(key, "value", 5*time.Second)

	err := DelKeyIfExists(key)
	assert.NoError(t, err)

	val, _ := GetKey(key)
	assert.Empty(t, val)

	// Case 2: Key 不存在
	err = DelKeyIfExists("nonexistent_key")
	assert.NoError(t, err)
}

func TestSetAndGetKey(t *testing.T) {
	key := "testkey"
	value := "testvalue"
	expiration := 1 * time.Second

	err := SetKeyEx(key, value, expiration)
	assert.NoError(t, err)

	val, err := GetKey(key)
	assert.NoError(t, err)
	assert.Equal(t, value, val)

	// 清理
	DelKeyIfExists(key)
}

func TestGetKey_NotExists(t *testing.T) {

	key := "nonexistentkey"
	val, err := GetKey(key)
	assert.NoError(t, err)
	assert.Empty(t, val)
}

func TestDelKeyIfExists_KeyExists(t *testing.T) {

	key := "deltest"
	SetKeyEx(key, "val", 5*time.Second)

	err := DelKeyIfExists(key)
	assert.NoError(t, err)

	// 再次获取应为空
	val, _ := GetKey(key)
	assert.Empty(t, val)
}

func TestGetKeyNilIsErr_KeyExists(t *testing.T) {
	key := "test_get_nil_is_err"
	SetKeyEx(key, "value", 5*time.Second)

	val, err := GetKeyNilIsErr(key)
	assert.NoError(t, err)
	assert.Equal(t, "value", val)

	DelKeyIfExists(key)
}

func TestGetKeyNilIsErr_KeyNotExists(t *testing.T) {
	key := "nonexistent_get_nil_is_err"
	val, err := GetKeyNilIsErr(key)
	assert.Error(t, err)
	assert.EqualError(t, err, redis.Nil.Error())
	assert.Empty(t, val)
}

func TestGetKeyWithPrefixNilIsErr_KeyFound(t *testing.T) {
	key := "prefix:test_key_123"
	SetKeyEx(key, "value", 5*time.Second)

	result, err := GetKeyWithPrefixNilIsErr("prefix:")
	assert.NoError(t, err)
	assert.Equal(t, key, result)

	DelKeyIfExists(key)
}

func TestGetKeyWithPrefixNilIsErr_NoKey(t *testing.T) {
	result, err := GetKeyWithPrefixNilIsErr("nonexistent_prefix:")
	assert.Error(t, err)
	assert.EqualError(t, err, redis.Nil.Error())
	assert.Empty(t, result)
}

func TestGetKeyWithPrefixNilIsErr_MultipleKeys(t *testing.T) {
	SetKeyEx("prefix:key1", "value1", 5*time.Second)
	SetKeyEx("prefix:key2", "value2", 5*time.Second)

	_, err := GetKeyWithPrefixNilIsErr("prefix:")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "找到了数量大于1的key")

	DelKeysWithPrefix("prefix:")
}

func TestGetKeyWithSuffixNilIsErr_KeyFound(t *testing.T) {
	key := "key_suffix:test"
	SetKeyEx(key, "value", 5*time.Second)

	result, err := GetKeyWithSuffixNilIsErr(":test")
	assert.NoError(t, err)
	assert.Equal(t, key, result)

	DelKeyIfExists(key)
}

func TestGetKeyWithSuffixNilIsErr_NoKey(t *testing.T) {
	result, err := GetKeyWithSuffixNilIsErr("_suffix:")
	assert.Error(t, err)
	assert.EqualError(t, err, redis.Nil.Error())
	assert.Empty(t, result)
}

func TestGetKeyWithSuffixNilIsErr_MultipleKeys(t *testing.T) {
	SetKeyEx("key1:suffix", "value1", 5*time.Second)
	SetKeyEx("key2:suffix", "value2", 5*time.Second)

	_, err := GetKeyWithSuffixNilIsErr(":suffix")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "找到了数量大于1的key")

	DelKeysWithSuffix(":suffix")
}

func TestDelKeysWithPattern_NoMatch(t *testing.T) {
	err := DelKeysWithPattern("nonexistent:*")
	assert.NoError(t, err)
}

func TestDelKeysWithPattern_Match(t *testing.T) {
	SetKeyEx("pattern:a", "1", 5*time.Second)
	SetKeyEx("pattern:b", "2", 5*time.Second)

	err := DelKeysWithPattern("pattern:*")
	assert.NoError(t, err)

	// 验证是否删除
	val, _ := GetKey("pattern:a")
	assert.Empty(t, val)
	val, _ = GetKey("pattern:b")
	assert.Empty(t, val)
}

func TestDelKeysWithPrefix(t *testing.T) {
	SetKeyEx("user:1", "alice", 5*time.Second)
	SetKeyEx("user:2", "bob", 5*time.Second)

	err := DelKeysWithPrefix("user:")
	assert.NoError(t, err)

	// 验证是否删除
	val, _ := GetKey("user:1")
	assert.Empty(t, val)
	val, _ = GetKey("user:2")
	assert.Empty(t, val)
}

func TestDelKeysWithSuffix(t *testing.T) {
	SetKeyEx("data:log1", "log_data1", 5*time.Second)
	SetKeyEx("data:log2", "log_data2", 5*time.Second)

	err := DelKeysWithSuffix(":log1")
	assert.NoError(t, err)

	// 验证是否删除
	val, _ := GetKey("data:log1")
	assert.Empty(t, val)
	val, _ = GetKey("data:log2") // 不该删
	assert.NotEmpty(t, val)
}

func TestDeleteAllRedisKeys(t *testing.T) {
	SetKeyEx("key1", "value1", 5*time.Second)
	SetKeyEx("key2", "value2", 5*time.Second)

	err := DeleteAllRedisKeys()
	assert.NoError(t, err)

	// 验证是否清空
	val, _ := GetKey("key1")
	assert.Empty(t, val)
	val, _ = GetKey("key2")
	assert.Empty(t, val)
}
