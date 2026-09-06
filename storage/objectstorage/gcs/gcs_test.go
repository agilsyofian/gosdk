package gcs_test

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"github.com/agilsyofian/gosdk/storage/objectstorage/gcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyLogger struct{}

func (d *dummyLogger) Info(ctx context.Context, msg string, payload logger.LogPayload)  {}
func (d *dummyLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {}
func (d *dummyLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {}
func (d *dummyLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload)  {}
func (d *dummyLogger) Fatal(ctx context.Context, msg string, payload logger.LogPayload) {}

func TestGCSInterfaceImplementation(t *testing.T) {
	var _ objectstorage.Storage = (*gcs.Storage)(nil)
	var _ health.Check = (*gcs.Storage)(nil)
}

func TestNewGCSStorage_Validation(t *testing.T) {
	ctx := context.Background()
	l := &dummyLogger{}

	// Missing bucket name
	_, err := gcs.NewGCSStorage(ctx, l, gcs.Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")
}

func TestDefaultConfig(t *testing.T) {
	cfg := gcs.DefaultConfig()
	assert.Empty(t, cfg.Bucket)
	assert.Empty(t, cfg.CredentialsJSON)
}
