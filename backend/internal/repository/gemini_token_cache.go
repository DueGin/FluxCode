package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DueGin/FluxCode/internal/service"
	"github.com/google/uuid"

	"github.com/redis/go-redis/v9"
)

const (
	geminiTokenKeyPrefix       = "gemini:token:"
	geminiRefreshLockKeyPrefix = "gemini:refresh_lock:"
)

var geminiRefreshLockReleaseScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

type geminiTokenCache struct {
	rdb        *redis.Client
	lockTokens sync.Map // key -> token
}

func NewGeminiTokenCache(rdb *redis.Client) service.GeminiTokenCache {
	return &geminiTokenCache{rdb: rdb}
}

func (c *geminiTokenCache) GetAccessToken(ctx context.Context, cacheKey string) (string, error) {
	key := fmt.Sprintf("%s%s", geminiTokenKeyPrefix, cacheKey)
	return c.rdb.Get(ctx, key).Result()
}

func (c *geminiTokenCache) SetAccessToken(ctx context.Context, cacheKey string, token string, ttl time.Duration) error {
	key := fmt.Sprintf("%s%s", geminiTokenKeyPrefix, cacheKey)
	return c.rdb.Set(ctx, key, token, ttl).Err()
}

func (c *geminiTokenCache) AcquireRefreshLock(ctx context.Context, cacheKey string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("%s%s", geminiRefreshLockKeyPrefix, cacheKey)
	token := uuid.NewString()
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, err
	}
	if ok {
		c.lockTokens.Store(key, token)
	}
	return ok, nil
}

func (c *geminiTokenCache) ReleaseRefreshLock(ctx context.Context, cacheKey string) error {
	key := fmt.Sprintf("%s%s", geminiRefreshLockKeyPrefix, cacheKey)
	tokenAny, ok := c.lockTokens.Load(key)
	if !ok {
		return nil
	}
	token, _ := tokenAny.(string)
	c.lockTokens.Delete(key)
	if token == "" {
		return nil
	}
	_, err := geminiRefreshLockReleaseScript.Run(ctx, c.rdb, []string{key}, token).Result()
	return err
}
