package s3_test

import (
	"context"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"github.com/agilsyofian/gosdk/storage/objectstorage/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyLogger struct{}

func (d *dummyLogger) Info(ctx context.Context, msg string, payload logger.LogPayload)  {}
func (d *dummyLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {}
func (d *dummyLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {}
func (d *dummyLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload)  {}
func (d *dummyLogger) Fatal(ctx context.Context, msg string, payload logger.LogPayload) {}

func TestS3InterfaceImplementation(t *testing.T) {
	var _ objectstorage.Storage = (*s3.Storage)(nil)
	var _ health.Check = (*s3.Storage)(nil)
}

func TestNewS3Storage_Validation(t *testing.T) {
	l := &dummyLogger{}

	// Missing endpoint
	_, err := s3.NewS3Storage(l, s3.Config{Bucket: "test-bucket"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint is required")

	// Missing bucket
	_, err = s3.NewS3Storage(l, s3.Config{Endpoint: "s3.amazonaws.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")
}

func TestNewS3Storage_SuccessInit(t *testing.T) {
	l := &dummyLogger{}
	reg := health.NewRegistry()

	cfg := s3.Config{
		Endpoint:         "s3.amazonaws.com",
		AccessKey:        "test-access-key",
		SecretKey:        "test-secret-key",
		Bucket:           "my-test-bucket",
		UseSSL:           true,
		Region:           "ap-southeast-1",
		AutoCreateBucket: false,
	}

	storage, err := s3.NewS3Storage(l, cfg, reg)
	require.NoError(t, err)
	require.NotNil(t, storage)
	assert.NotNil(t, storage.Client())

	// Close driver
	err = storage.Close(context.Background())
	assert.NoError(t, err)
}

func TestDefaultConfig(t *testing.T) {
	cfg := s3.DefaultConfig()
	assert.True(t, cfg.UseSSL)
	assert.False(t, cfg.AutoCreateBucket)
	assert.Equal(t, uint64(5*1024*1024), cfg.PartSize)
	assert.Equal(t, 30*time.Second, cfg.RequestTimeout)
}
