package service

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/stretchr/testify/assert"
)

func TestNotificationService_PushFCM(t *testing.T) {
	log := logger.NewZerologLogger(logger.Config{
		Output:              "stdout",
		LogLevel:            0,
		MaxLengthParamValue: 256,
	})

	svc := New(log)
	err := svc.PushFCM(context.Background(), "user-123", "user.created")
	assert.NoError(t, err)
}
