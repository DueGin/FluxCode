package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailQueueService_processTask_UnknownTaskTypeReturnsError(t *testing.T) {
	svc := &EmailQueueService{}

	err := svc.processTask(context.Background(), emailTaskPayload{
		TaskType: "verifycode_typo",
	})

	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnsupportedEmailTaskType)
}

