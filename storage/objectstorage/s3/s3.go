package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Ensure Storage implements objectstorage.Storage interface
var _ objectstorage.Storage = (*Storage)(nil)

// Ensure Storage implements health.Check interface
var _ health.Check = (*Storage)(nil)

const (
	attrRPCSystem  = "rpc.system"
	attrBlobBucket = "blob.bucket"
	attrBlobObject = "blob.object_name"
)

// Storage implements objectstorage.Storage for S3 and MinIO backends.
type Storage struct {
	client  *minio.Client
	bucket  string
	cfg     Config
	logger  logger.Logger
	appName string
	tracer  trace.Tracer
}

// NewS3Storage creates a new S3 / MinIO storage driver instance in Pure Go.
// If health.Registry is provided, it automatically registers a readiness probe for the S3 bucket.
func NewS3Storage(
	l logger.Logger,
	cfg Config,
	hrs ...health.Registry,
) (*Storage, error) {
	cfg = mergeConfig(DefaultConfig(), cfg)

	if cfg.Endpoint == "" {
		return nil, errors.New("s3: endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("s3: bucket name is required")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: failed to initialize client: %w", err)
	}

	ctx := context.Background()
	appName := coreContext.AppName(ctx)

	// Safe AutoCreateBucket: Only attempt creation if explicitly enabled.
	// This prevents AccessDenied errors in production environments where IAM users lack s3:CreateBucket permissions.
	if cfg.AutoCreateBucket {
		exists, err := client.BucketExists(ctx, cfg.Bucket)
		if err == nil && !exists {
			if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
				_ = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{})
			}
		}
	}

	s := &Storage{
		client:  client,
		bucket:  cfg.Bucket,
		cfg:     cfg,
		logger:  l,
		appName: appName,
		tracer:  otel.Tracer("gosdk/storage/objectstorage/s3"),
	}

	for _, hr := range hrs {
		if hr != nil {
			hr.RegisterReadiness("objectstorage_s3", s)
			break
		}
	}

	if l != nil {
		l.Info(ctx, "s3 storage driver initialized", logger.LogPayload{
			Type:      "STORAGE_S3",
			Method:    "START",
			IpAddress: cfg.Endpoint,
			AppName:   appName,
		})
	}

	return s, nil
}

// Client returns the underlying *minio.Client for direct advanced access.
func (s *Storage) Client() *minio.Client {
	return s.client
}

// Upload streams data from reader to the specified objectName.
func (s *Storage) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	objectName = filepath.Clean(objectName)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	ctx, span := s.tracer.Start(ctx, "s3.upload",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "s3"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String(attrBlobObject, objectName),
			attribute.Int64("blob.size", size),
			attribute.String("blob.content_type", contentType),
		),
	)
	defer span.End()

	opts := minio.PutObjectOptions{
		ContentType: contentType,
		PartSize:    s.cfg.PartSize,
	}

	_, err := s.client.PutObject(ctx, s.bucket, objectName, reader, size, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if s.logger != nil {
			s.logger.Error(ctx, fmt.Sprintf("s3: upload failed for object %s", objectName), logger.LogPayload{
				Type:   "STORAGE_S3",
				Method: "UPLOAD",
				Path:   objectName,
				Error:  err.Error(),
			})
		}
		return fmt.Errorf("s3: upload object '%s': %w", objectName, err)
	}

	span.SetStatus(codes.Ok, "success")
	if s.logger != nil && s.cfg.Logger.Enabled {
		s.logger.Info(ctx, fmt.Sprintf("s3: uploaded object %s", objectName), logger.LogPayload{
			Type:   "STORAGE_S3",
			Method: "UPLOAD",
			Path:   objectName,
		})
	}
	return nil
}

// Download streams the object content. The caller MUST close the returned io.ReadCloser.
func (s *Storage) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
	objectName = filepath.Clean(objectName)

	ctx, span := s.tracer.Start(ctx, "s3.download",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "s3"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String(attrBlobObject, objectName),
		),
	)
	defer span.End()

	obj, err := s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("s3: download object '%s': %w", objectName, err)
	}

	// Verify object can be accessed
	_, err = obj.Stat()
	if err != nil {
		_ = obj.Close()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("s3: stat object '%s': %w", objectName, err)
	}

	span.SetStatus(codes.Ok, "success")
	return obj, nil
}

// Delete removes an object from storage.
func (s *Storage) Delete(ctx context.Context, objectName string) error {
	objectName = filepath.Clean(objectName)

	ctx, span := s.tracer.Start(ctx, "s3.delete",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "s3"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String(attrBlobObject, objectName),
		),
	)
	defer span.End()

	err := s.client.RemoveObject(ctx, s.bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("s3: delete object '%s': %w", objectName, err)
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

// List returns all objects matching the given prefix.
func (s *Storage) List(ctx context.Context, prefix string) ([]objectstorage.ObjectInfo, error) {
	ctx, span := s.tracer.Start(ctx, "s3.list",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "s3"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String("blob.prefix", prefix),
		),
	)
	defer span.End()

	var result []objectstorage.ObjectInfo
	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			span.RecordError(obj.Err)
			span.SetStatus(codes.Error, obj.Err.Error())
			return nil, fmt.Errorf("s3: list objects error: %w", obj.Err)
		}
		result = append(result, objectstorage.ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			ContentType:  obj.ContentType,
		})
	}

	span.SetStatus(codes.Ok, "success")
	return result, nil
}

// PresignedURL generates a presigned GET URL for client-direct downloads.
func (s *Storage) PresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	objectName = filepath.Clean(objectName)

	ctx, span := s.tracer.Start(ctx, "s3.presigned_url",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "s3"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String(attrBlobObject, objectName),
			attribute.Float64("blob.expiry_sec", expiry.Seconds()),
		),
	)
	defer span.End()

	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, expiry, url.Values{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("s3: presigned url for '%s': %w", objectName, err)
	}

	span.SetStatus(codes.Ok, "success")
	return presignedURL.String(), nil
}

// Check implements health.Check for Kubernetes readiness verification.
func (s *Storage) Check(ctx context.Context) health.Health {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return health.DownWithDetails(map[string]any{
			"error":    err.Error(),
			"bucket":   s.bucket,
			"endpoint": s.cfg.Endpoint,
		})
	}
	if !exists {
		return health.DownWithDetails(map[string]any{
			"error":  "bucket does not exist",
			"bucket": s.bucket,
		})
	}

	return health.Up()
}

// Close gracefully stops the S3 storage driver.
func (s *Storage) Close(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Info(ctx, "closing s3 storage driver", logger.LogPayload{
			Type:   "STORAGE_S3",
			Method: "SHUTDOWN",
		})
	}
	return nil
}
