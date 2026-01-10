//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type adminAccountRepoUpdateStub struct {
	accountRepoStub

	account     *Account
	getErr      error
	updateErr   error
	updateCalls int
}

func (s *adminAccountRepoUpdateStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.account == nil {
		return nil, ErrAccountNotFound
	}
	return s.account, nil
}

func (s *adminAccountRepoUpdateStub) Update(ctx context.Context, account *Account) error {
	s.updateCalls++
	return s.updateErr
}

func TestAdminService_UpdateAccount_AllowsClearingProxyID(t *testing.T) {
	existingProxyID := int64(9)
	repo := &adminAccountRepoUpdateStub{
		account: &Account{
			ID:      1,
			Name:    "acc",
			ProxyID: &existingProxyID,
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	got, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		ProxyIDSet: true,
		ProxyID:    nil,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Nil(t, got.ProxyID)
	require.Equal(t, 1, repo.updateCalls)
}

func TestAdminService_UpdateAccount_DoesNotChangeProxyIDWhenNotProvided(t *testing.T) {
	existingProxyID := int64(9)
	repo := &adminAccountRepoUpdateStub{
		account: &Account{
			ID:      1,
			Name:    "acc",
			ProxyID: &existingProxyID,
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	got, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.ProxyID)
	require.Equal(t, existingProxyID, *got.ProxyID)
	require.Equal(t, 1, repo.updateCalls)
}
