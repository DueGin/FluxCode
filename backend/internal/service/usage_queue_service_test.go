package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestParseUsageQueuePayload_DefaultsRequestIDFromPayloadID(t *testing.T) {
	payload := map[string]any{
		"id":         "task-1",
		"kind":       "claude",
		"request_id": "",
		"model":      "claude-3-7-sonnet",
		"user_id":    1,
		"api_key_id": 2,
		"account_id": 3,
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)

	msg := redis.XMessage{
		ID: "1700000000000-0",
		Values: map[string]any{
			"payload": string(b),
		},
	}

	got, err := parseUsageQueuePayload(msg)
	require.NoError(t, err)
	require.Equal(t, "task-1", got.ID)
	require.Equal(t, "task-1", got.RequestID)
}

func TestParseUsageQueuePayload_HashesOverlongRequestID(t *testing.T) {
	long := strings.Repeat("x", maxUsageRequestIDLen+10)
	payload := map[string]any{
		"id":         "task-2",
		"kind":       "claude",
		"request_id": long,
		"model":      "claude-3-7-sonnet",
		"user_id":    1,
		"api_key_id": 2,
		"account_id": 3,
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)

	msg := redis.XMessage{
		ID: "1700000000000-0",
		Values: map[string]any{
			"payload": string(b),
		},
	}

	got, err := parseUsageQueuePayload(msg)
	require.NoError(t, err)
	require.Len(t, got.RequestID, maxUsageRequestIDLen)
	require.NotEqual(t, long, got.RequestID)
}
