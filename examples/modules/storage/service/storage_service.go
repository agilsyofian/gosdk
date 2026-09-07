package service

import (
	"context"
	"io"
	"time"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/storage/domain"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
)

type IStorageService interface {
	Upload(ctx context.Context, cmd domain.UploadFileCmd) (*domain.UploadedFile, error)
	Download(ctx context.Context, objectName string) (io.ReadCloser, error)
	PresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
}

const (
	ErrorStorageClientNotConfigure = "storage client is not configured"
	ErrorObjectStorageUnavailable  = "object storage unavailable"
	ErrorInvalidObjectName         = "invalid object name"
)

type StorageService struct {
	appConfig config.AppConfig
	logger    logger.Logger
	storage   objectstorage.Storage
}

func New(deps StorageServiceDeps) IStorageService {
	return &StorageService{
		appConfig: deps.AppConfig,
		logger:    deps.Logger,
		storage:   deps.Storage,
	}
}

func (s *StorageService) Upload(ctx context.Context, cmd domain.UploadFileCmd) (*domain.UploadedFile, error) {
	if s.storage == nil {
		return nil, errors.InternalServerError(errors.String(ErrorStorageClientNotConfigure), s.appConfig.AppName, "5001", ErrorObjectStorageUnavailable)
	}

	if cmd.ObjectName == "" {
		return nil, errors.BadRequest(errors.String("object name cannot be empty"), s.appConfig.AppName, "4001", ErrorInvalidObjectName)
	}

	if cmd.ContentType == "" {
		cmd.ContentType = "application/octet-stream"
	}

	err := s.storage.Upload(ctx, cmd.ObjectName, cmd.Content, cmd.Size, cmd.ContentType)
	if err != nil {
		return nil, errors.InternalServerError(err, s.appConfig.AppName, "5002", "failed to upload object")
	}

	presignedURL, _ := s.storage.PresignedURL(ctx, cmd.ObjectName, 15*time.Minute)

	return &domain.UploadedFile{
		ObjectName:   cmd.ObjectName,
		PresignedURL: presignedURL,
	}, nil
}

func (s *StorageService) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
	if s.storage == nil {
		return nil, errors.InternalServerError(errors.String(ErrorStorageClientNotConfigure), s.appConfig.AppName, "5001", ErrorObjectStorageUnavailable)
	}

	if objectName == "" {
		return nil, errors.BadRequest(errors.String("object name is required"), s.appConfig.AppName, "4001", ErrorInvalidObjectName)
	}

	stream, err := s.storage.Download(ctx, objectName)
	if err != nil {
		return nil, errors.NotFound(err, s.appConfig.AppName, "4004", "object not found")
	}

	return stream, nil
}

func (s *StorageService) PresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if s.storage == nil {
		return "", errors.InternalServerError(errors.String(ErrorStorageClientNotConfigure), s.appConfig.AppName, "5001", ErrorObjectStorageUnavailable)
	}

	if objectName == "" {
		return "", errors.BadRequest(errors.String("object name is required"), s.appConfig.AppName, "4001", ErrorInvalidObjectName)
	}

	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	url, err := s.storage.PresignedURL(ctx, objectName, expiry)
	if err != nil {
		return "", errors.InternalServerError(err, s.appConfig.AppName, "5003", "failed to generate presigned URL")
	}

	return url, nil
}
