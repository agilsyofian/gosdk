package objectstorage

import (
	"context"
	"io"
	"time"
)

// ObjectInfo holds metadata information about an object in storage.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
}

// Storage defines a cloud-agnostic interface for object storage operations (S3, MinIO, GCS, etc.).
type Storage interface {
	// Upload streams data from reader to the specified objectName.
	Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error

	// Download retrieves the object stream. The caller is responsible for closing the returned ReadCloser.
	Download(ctx context.Context, objectName string) (io.ReadCloser, error)

	// Delete removes the specified object from storage.
	Delete(ctx context.Context, objectName string) error

	// List returns a list of objects matching the specified prefix.
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)

	// PresignedURL generates a pre-signed URL for direct client access with the given expiration.
	PresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)

	// Close releases any resources held by the storage client.
	Close(ctx context.Context) error
}
