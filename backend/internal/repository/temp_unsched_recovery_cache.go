package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/DueGin/FluxCode/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	tempUnschedRecoveryPendingKey    = "temp_unsched_recovery:pending"
	tempUnschedRecoveryProcessingKey = "temp_unsched_recovery:processing"
)

var (
	tempUnschedRecoveryClaimScript = redis.NewScript(`
		local pending = KEYS[1]
		local processing = KEYS[2]
		local now = tonumber(ARGV[1])
		local limit = tonumber(ARGV[2])
		local lease = tonumber(ARGV[3])

		if not limit or limit <= 0 then
			return {}
		end

		local items = redis.call('ZRANGEBYSCORE', pending, '-inf', now, 'LIMIT', 0, limit)
		if #items == 0 then
			return {}
		end

		redis.call('ZREM', pending, unpack(items))
		local leaseUntil = now + lease
		for _, member in ipairs(items) do
			redis.call('ZADD', processing, leaseUntil, member)
		end

		return items
	`)

	tempUnschedRecoveryRequeueExpiredScript = redis.NewScript(`
		local processing = KEYS[1]
		local pending = KEYS[2]
		local now = tonumber(ARGV[1])
		local limit = tonumber(ARGV[2])
		local retryDelay = tonumber(ARGV[3])

		if not limit or limit <= 0 then
			return {}
		end

		local items = redis.call('ZRANGEBYSCORE', processing, '-inf', now, 'LIMIT', 0, limit)
		if #items == 0 then
			return {}
		end

		redis.call('ZREM', processing, unpack(items))
		local retryAt = now + retryDelay
		for _, member in ipairs(items) do
			redis.call('ZADD', pending, retryAt, member)
		end

		return items
	`)

	tempUnschedRecoveryRequeueScript = redis.NewScript(`
		local pending = KEYS[1]
		local processing = KEYS[2]
		local score = tonumber(ARGV[1])
		local member = ARGV[2]

		redis.call('ZREM', processing, member)
		redis.call('ZADD', pending, score, member)
		return 1
	`)

	tempUnschedRecoveryAckScript = redis.NewScript(`
		local pending = KEYS[1]
		local processing = KEYS[2]
		local member = ARGV[1]

		redis.call('ZREM', processing, member)
		redis.call('ZREM', pending, member)
		return 1
	`)
)

type tempUnschedRecoveryCache struct {
	rdb *redis.Client
}

func NewTempUnschedRecoveryCache(rdb *redis.Client) service.TempUnschedRecoveryCache {
	return &tempUnschedRecoveryCache{rdb: rdb}
}

func (c *tempUnschedRecoveryCache) AddCandidates(ctx context.Context, entries []service.TempUnschedRecoveryEntry) error {
	if c == nil || c.rdb == nil || len(entries) == 0 {
		return nil
	}
	members := make([]redis.Z, 0, len(entries))
	for _, entry := range entries {
		if entry.AccountID <= 0 || entry.Until.IsZero() {
			continue
		}
		score := float64(entry.Until.Unix())
		if score <= 0 {
			continue
		}
		members = append(members, redis.Z{
			Score:  score,
			Member: strconv.FormatInt(entry.AccountID, 10),
		})
	}
	if len(members) == 0 {
		return nil
	}
	return c.rdb.ZAdd(ctx, tempUnschedRecoveryPendingKey, members...).Err()
}

func (c *tempUnschedRecoveryCache) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]int64, error) {
	if c == nil || c.rdb == nil || limit <= 0 {
		return nil, nil
	}
	leaseSeconds := int64(lease.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = int64((5 * time.Minute).Seconds())
	}
	nowUnix := now.Unix()
	if nowUnix <= 0 {
		return nil, nil
	}
	result, err := tempUnschedRecoveryClaimScript.Run(
		ctx,
		c.rdb,
		[]string{tempUnschedRecoveryPendingKey, tempUnschedRecoveryProcessingKey},
		nowUnix,
		limit,
		leaseSeconds,
	).Result()
	if err != nil {
		return nil, err
	}
	return parseTempUnschedRecoveryIDs(result)
}

func (c *tempUnschedRecoveryCache) RequeueExpired(ctx context.Context, now time.Time, retryDelay time.Duration, limit int) (int, error) {
	if c == nil || c.rdb == nil || limit <= 0 {
		return 0, nil
	}
	retrySeconds := int64(retryDelay.Seconds())
	if retrySeconds <= 0 {
		retrySeconds = int64((5 * time.Minute).Seconds())
	}
	nowUnix := now.Unix()
	if nowUnix <= 0 {
		return 0, nil
	}
	result, err := tempUnschedRecoveryRequeueExpiredScript.Run(
		ctx,
		c.rdb,
		[]string{tempUnschedRecoveryProcessingKey, tempUnschedRecoveryPendingKey},
		nowUnix,
		limit,
		retrySeconds,
	).Result()
	if err != nil {
		return 0, err
	}
	ids, err := parseTempUnschedRecoveryIDs(result)
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (c *tempUnschedRecoveryCache) Requeue(ctx context.Context, accountID int64, until time.Time) error {
	if c == nil || c.rdb == nil || accountID <= 0 || until.IsZero() {
		return nil
	}
	score := until.Unix()
	if score <= 0 {
		return nil
	}
	_, err := tempUnschedRecoveryRequeueScript.Run(
		ctx,
		c.rdb,
		[]string{tempUnschedRecoveryPendingKey, tempUnschedRecoveryProcessingKey},
		score,
		strconv.FormatInt(accountID, 10),
	).Result()
	return err
}

func (c *tempUnschedRecoveryCache) Ack(ctx context.Context, accountID int64) error {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return nil
	}
	_, err := tempUnschedRecoveryAckScript.Run(
		ctx,
		c.rdb,
		[]string{tempUnschedRecoveryPendingKey, tempUnschedRecoveryProcessingKey},
		strconv.FormatInt(accountID, 10),
	).Result()
	return err
}

func parseTempUnschedRecoveryIDs(result any) ([]int64, error) {
	if result == nil {
		return nil, nil
	}
	switch v := result.(type) {
	case []interface{}:
		ids := make([]int64, 0, len(v))
		for _, item := range v {
			id, err := parseTempUnschedRecoveryID(item)
			if err != nil {
				return nil, err
			}
			if id > 0 {
				ids = append(ids, id)
			}
		}
		return ids, nil
	case []string:
		ids := make([]int64, 0, len(v))
		for _, item := range v {
			id, err := strconv.ParseInt(item, 10, 64)
			if err != nil {
				return nil, err
			}
			if id > 0 {
				ids = append(ids, id)
			}
		}
		return ids, nil
	default:
		return nil, fmt.Errorf("unexpected result type %T", result)
	}
}

func parseTempUnschedRecoveryID(value any) (int64, error) {
	switch v := value.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected id type %T", value)
	}
}
