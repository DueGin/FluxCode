package service

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRequestID_Empty(t *testing.T) {
	id := normalizeRequestID("")
	require.NotEmpty(t, id)
	require.LessOrEqual(t, len(id), maxUsageRequestIDLen)
}

func TestNormalizeRequestID_Trim(t *testing.T) {
	require.Equal(t, "abc", normalizeRequestID("  abc  "))
}

func TestNormalizeRequestID_TooLong(t *testing.T) {
	long := strings.Repeat("a", maxUsageRequestIDLen+10)
	id := normalizeRequestID(long)
	require.Len(t, id, maxUsageRequestIDLen)
	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), id)
}

func TestNormalizeRequestIDWithFallback(t *testing.T) {
	require.Equal(t, "fallback", normalizeRequestIDWithFallback("", "fallback"))
}
