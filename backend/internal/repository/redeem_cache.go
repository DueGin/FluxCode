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
	redeemRateLimitKeyPrefix = "redeem:ratelimit:"
	redeemLockKeyPrefix      = "redeem:lock:"
	redeemRateLimitDuration  = 24 * time.Hour
)

var redeemLockReleaseScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

// redeemRateLimitKey generates the Redis key for redeem attempt rate limiting.
func redeemRateLimitKey(userID int64) string {
	return fmt.Sprintf("%s%d", redeemRateLimitKeyPrefix, userID)
}

// redeemLockKey generates the Redis key for redeem code locking.
func redeemLockKey(code string) string {
	return redeemLockKeyPrefix + code
}

type redeemCache struct {
	rdb        *redis.Client
	lockTokens sync.Map // key -> token
}

func NewRedeemCache(rdb *redis.Client) service.RedeemCache {
	return &redeemCache{rdb: rdb}
}

func (c *redeemCache) GetRedeemAttemptCount(ctx context.Context, userID int64) (int, error) {
	key := redeemRateLimitKey(userID)
	count, err := c.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

func (c *redeemCache) IncrementRedeemAttemptCount(ctx context.Context, userID int64) error {
	key := redeemRateLimitKey(userID)
	pipe := c.rdb.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, redeemRateLimitDuration)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *redeemCache) AcquireRedeemLock(ctx context.Context, code string, ttl time.Duration) (bool, error) {
	key := redeemLockKey(code)
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

func (c *redeemCache) ReleaseRedeemLock(ctx context.Context, code string) error {
	key := redeemLockKey(code)
	tokenAny, ok := c.lockTokens.Load(key)
	if !ok {
		return nil
	}
	token, _ := tokenAny.(string)
	c.lockTokens.Delete(key)
	if token == "" {
		return nil
	}
	_, err := redeemLockReleaseScript.Run(ctx, c.rdb, []string{key}, token).Result()
	return err
}
