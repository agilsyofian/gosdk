package gcs

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
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

// Storage implements objectstorage.Storage for Google Cloud Storage.
type Storage struct {
	client  *storage.Client
	bucket  string
	cfg     Config
	logger  logger.Logger
	appName string
	tracer  trace.Tracer
}

// NewGCSStorage creates a new Google Cloud Storage driver instance in Pure Go.
// Supports Service Account JSON (raw or base64), file path, or Workload Identity / ADC.
func NewGCSStorage(
	ctx context.Context,
	l logger.Logger,
	cfg Config,
	hrs ...health.Registry,
) (*Storage, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("gcs: bucket name is required")
	}

	var opts []option.ClientOption

	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	} else if cfg.CredentialsJSON != "" {
		var credBytes []byte
		// Check if credentials are base64 encoded
		if decoded, err := base64.StdEncoding.DecodeString(cfg.CredentialsJSON); err == nil && len(decoded) > 0 {
			credBytes = decoded
		} else {
			credBytes = []byte(cfg.CredentialsJSON)
		}
		opts = append(opts, option.WithCredentialsJSON(credBytes))
	}
	// If no credentials provided, Google SDK defaults to Application Default Credentials (ADC) / Workload Identity.

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gcs: failed to initialize client: %w", err)
	}

	appName := coreContext.AppName(ctx)

	s := &Storage{
		client:  client,
		bucket:  cfg.Bucket,
		cfg:     cfg,
		logger:  l,
		appName: appName,
		tracer:  otel.Tracer("gosdk/storage/objectstorage/gcs"),
	}

	for _, hr := range hrs {
		if hr != nil {
			hr.RegisterReadiness("objectstorage_gcs", s)
			break
		}
	}

	if l != nil {
		l.Info(ctx, "gcs storage driver initialized", logger.LogPayload{
			Type:    "STORAGE_GCS",
			Method:  "START",
			AppName: appName,
		})
	}

	return s, nil
}

// Client returns the underlying *storage.Client for direct Google Cloud Storage access.
func (s *Storage) Client() *storage.Client {
	return s.client
}

// Upload streams data from reader to the specified objectName.
func (s *Storage) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	objectName = filepath.Clean(objectName)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	ctx, span := s.tracer.Start(ctx, "gcs.upload",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "gcs"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String(attrBlobObject, objectName),
			attribute.Int64("blob.size", size),
			attribute.String("blob.content_type", contentType),
		),
	)
	defer span.End()

	obj := s.client.Bucket(s.bucket).Object(objectName)
	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	var copyErr error
	if size > 0 {
		_, copyErr = io.CopyN(writer, reader, size)
	} else {
		_, copyErr = io.Copy(writer, reader)
	}

	closeErr := writer.Close()
	if copyErr != nil {
		span.RecordError(copyErr)
		span.SetStatus(codes.Error, copyErr.Error())
		return fmt.Errorf("gcs: upload copy error '%s': %w", objectName, copyErr)
	}
	if closeErr != nil {
		span.RecordError(closeErr)
		span.SetStatus(codes.Error, closeErr.Error())
		return fmt.Errorf("gcs: upload close error '%s': %w", objectName, closeErr)
	}

	span.SetStatus(codes.Ok, "success")
	if s.logger != nil && s.cfg.Logger.Enabled {
		s.logger.Info(ctx, fmt.Sprintf("gcs: uploaded object %s", objectName), logger.LogPayload{
			Type:   "STORAGE_GCS",
			Method: "UPLOAD",
			Path:   objectName,
		})
	}
	return nil
}

// Download streams the object content. The caller MUST close the returned io.ReadCloser.
func (s *Storage) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
	objectName = filepath.Clean(objectName)

	ctx, span := s.tracer.Start(ctx, "gcs.download",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "gcs"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String(attrBlobObject, objectName),
		),
	)
	defer span.End()

	reader, err := s.client.Bucket(s.bucket).Object(objectName).NewReader(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("gcs: download object '%s': %w", objectName, err)
	}

	span.SetStatus(codes.Ok, "success")
	return reader, nil
}

// Delete removes an object from GCS.
func (s *Storage) Delete(ctx context.Context, objectName string) error {
	objectName = filepath.Clean(objectName)

	ctx, span := s.tracer.Start(ctx, "gcs.delete",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "gcs"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String(attrBlobObject, objectName),
		),
	)
	defer span.End()

	err := s.client.Bucket(s.bucket).Object(objectName).Delete(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("gcs: delete object '%s': %w", objectName, err)
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

// List returns all objects matching the given prefix.
func (s *Storage) List(ctx context.Context, prefix string) ([]objectstorage.ObjectInfo, error) {
	ctx, span := s.tracer.Start(ctx, "gcs.list",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "gcs"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String("blob.prefix", prefix),
		),
	)
	defer span.End()

	var result []objectstorage.ObjectInfo
	it := s.client.Bucket(s.bucket).Objects(ctx, &storage.Query{Prefix: prefix})

	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("gcs: list objects error: %w", err)
		}

		result = append(result, objectstorage.ObjectInfo{
			Key:          attrs.Name,
			Size:         attrs.Size,
			LastModified: attrs.Updated,
			ETag:         attrs.Etag,
			ContentType:  attrs.ContentType,
		})
	}

	span.SetStatus(codes.Ok, "success")
	return result, nil
}

// PresignedURL generates a signed GET URL for client-direct access.
func (s *Storage) PresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	objectName = filepath.Clean(objectName)

	ctx, span := s.tracer.Start(ctx, "gcs.presigned_url",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrRPCSystem, "gcs"),
			attribute.String(attrBlobBucket, s.bucket),
			attribute.String(attrBlobObject, objectName),
			attribute.Float64("blob.expiry_sec", expiry.Seconds()),
		),
	)
	defer span.End()

	url, err := s.client.Bucket(s.bucket).SignedURL(objectName, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("gcs: presigned url for '%s': %w", objectName, err)
	}

	span.SetStatus(codes.Ok, "success")
	return url, nil
}

// Check implements health.Check for Kubernetes readiness verification.
func (s *Storage) Check(ctx context.Context) health.Health {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := s.client.Bucket(s.bucket).Attrs(ctx)
	if err != nil {
		return health.DownWithDetails(map[string]any{
			"error":  err.Error(),
			"bucket": s.bucket,
		})
	}

	return health.Up()
}

// Close gracefully releases GCS client connections.
func (s *Storage) Close(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Info(ctx, "closing gcs storage driver", logger.LogPayload{
			Type:   "STORAGE_GCS",
			Method: "SHUTDOWN",
		})
	}
	return s.client.Close()
}
