//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sqlExecSpy struct {
	execCalls int
	lastQuery string
	lastArgs  []any
}

func (s *sqlExecSpy) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	s.execCalls++
	s.lastQuery = query
	s.lastArgs = args
	return sqlResultStub{}, nil
}

func (s *sqlExecSpy) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	panic("unexpected QueryContext call")
}

type sqlResultStub struct{}

func (sqlResultStub) LastInsertId() (int64, error) { return 0, nil }
func (sqlResultStub) RowsAffected() (int64, error) { return 1, nil }

func TestAccountRepository_SetRateLimited_IsExtendOnlyInDB(t *testing.T) {
	spy := &sqlExecSpy{}
	repo := newAccountRepositoryWithSQL(nil, spy)

	resetAt := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	require.NotPanics(t, func() {
		require.NoError(t, repo.SetRateLimited(context.Background(), 1, resetAt))
	})

	require.Equal(t, 1, spy.execCalls)
	require.Contains(t, spy.lastQuery, "rate_limit_reset_at")
	require.Contains(t, spy.lastQuery, "rate_limit_reset_at <")
	require.Contains(t, spy.lastQuery, "CASE")
	require.Len(t, spy.lastArgs, 2)
	require.Equal(t, resetAt, spy.lastArgs[0])
	require.Equal(t, int64(1), spy.lastArgs[1])
}

