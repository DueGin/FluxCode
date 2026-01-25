package service

import (
	"context"
	"time"
)

// SettingCache is a Redis-backed cache for system settings.
//
// Note: This cache is used as a read-optimized mirror (DB remains source of truth).
// Missing keys should return ErrSettingNotFound to allow fallback to DB/defaults.
type SettingCache interface {
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetMultiple(ctx context.Context, keys []string) (map[string]string, error)
	SetMultiple(ctx context.Context, settings map[string]string) error

	AcquireUpdateLock(ctx context.Context, ttl time.Duration) (token string, ok bool, err error)
	ReleaseUpdateLock(ctx context.Context, token string) error
}
