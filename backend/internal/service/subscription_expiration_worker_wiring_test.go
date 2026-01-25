package service

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionExpirationWorkerWiring(t *testing.T) {
	t.Parallel()

	content := string(mustReadSiblingFile(t, "wire.go"))
	require.Contains(t, content, "ProvideSubscriptionExpirationWorker", "需要在 service/wire.go 中注入并启动订阅过期修复 worker")
}

func TestSubscriptionExpirationWorkerCleanupWiring(t *testing.T) {
	t.Parallel()

	// backend/internal/service -> backend/cmd/server/wire.go
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	serviceDir := filepath.Dir(thisFile)
	wirePath := filepath.Clean(filepath.Join(serviceDir, "..", "..", "cmd", "server", "wire.go"))

	content, err := os.ReadFile(wirePath)
	require.NoError(t, err, "读取 %s 失败", wirePath)
	require.Contains(t, string(content), "SubscriptionExpirationWorker", "需要在 server cleanup 中 Stop 订阅过期修复 worker")
}

func TestSubscriptionExpirationWorkerImplementationExists(t *testing.T) {
	t.Parallel()

	content := string(mustReadSiblingFile(t, "subscription_expiration_worker.go"))
	require.Contains(t, content, "pg_try_advisory_lock", "多机部署需要使用 advisory lock 保证单实例执行")
	require.Contains(t, content, "user_subscriptions", "需要更新 user_subscriptions 状态")
}

func mustReadSiblingFile(t *testing.T, name string) []byte {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)
	path := filepath.Join(dir, name)
	content, err := os.ReadFile(path)
	require.NoError(t, err, "读取 %s 失败", path)
	return content
}

