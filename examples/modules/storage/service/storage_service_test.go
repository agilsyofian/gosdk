package service

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/storage/domain"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"github.com/stretchr/testify/assert"
)

type mockStorage struct {
	uploadedName string
	uploadedData []byte
	uploadErr    error
	downloadData []byte
	downloadErr  error
	presignedURL string
	presignedErr error
}

func (m *mockStorage) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	m.uploadedName = objectName
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(reader)
	m.uploadedData = buf.Bytes()
	return m.uploadErr
}

func (m *mockStorage) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
	if m.downloadErr != nil {
		return nil, m.downloadErr
	}
	return io.NopCloser(bytes.NewReader(m.downloadData)), nil
}

func (m *mockStorage) Delete(ctx context.Context, objectName string) error {
	return nil
}

func (m *mockStorage) List(ctx context.Context, prefix string) ([]objectstorage.ObjectInfo, error) {
	return nil, nil
}

func (m *mockStorage) PresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	return m.presignedURL, m.presignedErr
}

func (m *mockStorage) Close(ctx context.Context) error {
	return nil
}

func generateTestLogger() logger.Logger {
	return logger.NewZerologLogger(logger.Config{
		Output:              "stdout",
		LogLevel:            0,
		MaxLengthParamValue: 256,
	})
}

func TestStorageService_UploadSuccess(t *testing.T) {
	mock := &mockStorage{
		presignedURL: "https://storage.example.com/test.txt",
	}

	svc := New(StorageServiceDeps{
		AppConfig: config.AppConfig{AppName: "test-app"},
		Logger:    generateTestLogger(),
		Storage:   mock,
	})

	res, err := svc.Upload(context.Background(), domain.UploadFileCmd{
		ObjectName:  "test.txt",
		Content:     strings.NewReader("hello storage"),
		Size:        13,
		ContentType: "text/plain",
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "test.txt", res.ObjectName)
	assert.Equal(t, "https://storage.example.com/test.txt", res.PresignedURL)
	assert.Equal(t, "hello storage", string(mock.uploadedData))
}

func TestStorageService_DownloadSuccess(t *testing.T) {
	mock := &mockStorage{
		downloadData: []byte("file content"),
	}

	svc := New(StorageServiceDeps{
		AppConfig: config.AppConfig{AppName: "test-app"},
		Logger:    generateTestLogger(),
		Storage:   mock,
	})

	rc, err := svc.Download(context.Background(), "test.txt")
	assert.NoError(t, err)
	assert.NotNil(t, rc)

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(rc)
	_ = rc.Close()
	assert.Equal(t, "file content", buf.String())
}

func TestStorageService_NilStorageFails(t *testing.T) {
	svc := New(StorageServiceDeps{
		AppConfig: config.AppConfig{AppName: "test-app"},
		Logger:    generateTestLogger(),
		Storage:   nil,
	})

	_, err := svc.Upload(context.Background(), domain.UploadFileCmd{ObjectName: "foo"})
	assert.Error(t, err)

	_, err = svc.Download(context.Background(), "foo")
	assert.Error(t, err)

	_, err = svc.PresignedURL(context.Background(), "foo", time.Minute)
	assert.Error(t, err)
}
