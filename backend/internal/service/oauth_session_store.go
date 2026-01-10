package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// OAuthSessionStore is a cross-instance session store for OAuth flows.
// It is used to persist short-lived OAuth state (PKCE verifier, redirect URI, etc.)
// so that the flow can survive load balancing without relying on sticky sessions.
type OAuthSessionStore[T any] interface {
	Set(ctx context.Context, sessionID string, session *T) error
	Get(ctx context.Context, sessionID string) (*T, bool, error)
	Delete(ctx context.Context, sessionID string) error
	Stop()
}

// InMemoryOAuthSessionStore represents the legacy in-process session store API.
// This adapter allows us to keep the original in-memory implementation as a fallback.
type InMemoryOAuthSessionStore[T any] interface {
	Set(sessionID string, session *T)
	Get(sessionID string) (*T, bool)
	Delete(sessionID string)
	Stop()
}

type inMemoryOAuthSessionStoreAdapter[T any] struct {
	store InMemoryOAuthSessionStore[T]
}

func NewInMemoryOAuthSessionStoreAdapter[T any](store InMemoryOAuthSessionStore[T]) OAuthSessionStore[T] {
	return &inMemoryOAuthSessionStoreAdapter[T]{store: store}
}

func (s *inMemoryOAuthSessionStoreAdapter[T]) Set(_ context.Context, sessionID string, session *T) error {
	s.store.Set(sessionID, session)
	return nil
}

func (s *inMemoryOAuthSessionStoreAdapter[T]) Get(_ context.Context, sessionID string) (*T, bool, error) {
	session, ok := s.store.Get(sessionID)
	return session, ok, nil
}

func (s *inMemoryOAuthSessionStoreAdapter[T]) Delete(_ context.Context, sessionID string) error {
	s.store.Delete(sessionID)
	return nil
}

func (s *inMemoryOAuthSessionStoreAdapter[T]) Stop() {
	s.store.Stop()
}

// RedisJSONOAuthSessionStore stores sessions as JSON with a key TTL.
// Key format: <keyPrefix><sessionID>
type RedisJSONOAuthSessionStore[T any] struct {
	rdb       *redis.Client
	keyPrefix string
	ttl       time.Duration
}

func NewRedisJSONOAuthSessionStore[T any](rdb *redis.Client, keyPrefix string, ttl time.Duration) *RedisJSONOAuthSessionStore[T] {
	return &RedisJSONOAuthSessionStore[T]{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		ttl:       ttl,
	}
}

func (s *RedisJSONOAuthSessionStore[T]) Set(ctx context.Context, sessionID string, session *T) error {
	if s.rdb == nil {
		return errors.New("redis client is nil")
	}
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("sessionID is empty")
	}
	if session == nil {
		return errors.New("session is nil")
	}

	b, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	key := s.keyPrefix + sessionID
	if err := s.rdb.Set(ctx, key, b, s.ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

func (s *RedisJSONOAuthSessionStore[T]) Get(ctx context.Context, sessionID string) (*T, bool, error) {
	if s.rdb == nil {
		return nil, false, errors.New("redis client is nil")
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, false, errors.New("sessionID is empty")
	}

	key := s.keyPrefix + sessionID
	b, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("redis get %q: %w", key, err)
	}

	var session T
	if err := json.Unmarshal(b, &session); err != nil {
		return nil, false, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, true, nil
}

func (s *RedisJSONOAuthSessionStore[T]) Delete(ctx context.Context, sessionID string) error {
	if s.rdb == nil {
		return errors.New("redis client is nil")
	}
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("sessionID is empty")
	}

	key := s.keyPrefix + sessionID
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis del %q: %w", key, err)
	}
	return nil
}

func (s *RedisJSONOAuthSessionStore[T]) Stop() {}

