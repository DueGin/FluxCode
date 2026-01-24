package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DueGin/FluxCode/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const systemSettingCacheHashKey = "settings:system"
const systemSettingUpdateLockKey = "settings:system:update_lock"

var settingUpdateLockReleaseScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

type settingCache struct {
	rdb *redis.Client
}

func NewSettingCache(rdb *redis.Client) service.SettingCache {
	return &settingCache{rdb: rdb}
}

func (c *settingCache) GetValue(ctx context.Context, key string) (string, error) {
	if c == nil || c.rdb == nil {
		return "", service.ErrSettingNotFound
	}
	val, err := c.rdb.HGet(ctx, systemSettingCacheHashKey, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", service.ErrSettingNotFound
		}
		return "", err
	}
	return val, nil
}

func (c *settingCache) Set(ctx context.Context, key, value string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.HSet(ctx, systemSettingCacheHashKey, key, value).Err()
}

func (c *settingCache) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	if c == nil || c.rdb == nil {
		return map[string]string{}, nil
	}

	values, err := c.rdb.HMGet(ctx, systemSettingCacheHashKey, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(keys))
	for i := range keys {
		if i >= len(values) {
			break
		}
		if values[i] == nil {
			continue
		}
		result[keys[i]] = fmt.Sprintf("%v", values[i])
	}
	return result, nil
}

func (c *settingCache) SetMultiple(ctx context.Context, settings map[string]string) error {
	if len(settings) == 0 {
		return nil
	}
	if c == nil || c.rdb == nil {
		return nil
	}
	fields := make(map[string]any, len(settings))
	for k, v := range settings {
		fields[k] = v
	}
	return c.rdb.HSet(ctx, systemSettingCacheHashKey, fields).Err()
}

func (c *settingCache) AcquireUpdateLock(ctx context.Context, ttl time.Duration) (string, bool, error) {
	if c == nil || c.rdb == nil {
		return "", true, nil
	}
	token := uuid.NewString()
	ok, err := c.rdb.SetNX(ctx, systemSettingUpdateLockKey, token, ttl).Result()
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return token, true, nil
}

func (c *settingCache) ReleaseUpdateLock(ctx context.Context, token string) error {
	if c == nil || c.rdb == nil {
		return nil
	}

	if token == "" {
		return nil
	}

	_, err := settingUpdateLockReleaseScript.Run(ctx, c.rdb, []string{systemSettingUpdateLockKey}, token).Result()
	return err
}
