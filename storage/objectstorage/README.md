# storage/objectstorage

Cloud-agnostic object storage interface for the Go SDK.

## Architecture

This package provides a unified [`objectstorage.Storage`](storage.go) interface with multiple cloud backend drivers:

- **[`storage/objectstorage/s3`](s3/)**: S3-compatible driver (AWS S3, MinIO, Ceph, Cloudflare R2).
- **[`storage/objectstorage/gcs`](gcs/)**: Google Cloud Storage driver (Workload Identity, ADC, Service Account).

---

## Interface Definition

```go
type Storage interface {
    Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error
    Download(ctx context.Context, objectName string) (io.ReadCloser, error)
    Delete(ctx context.Context, objectName string) error
    List(ctx context.Context, prefix string) ([]ObjectInfo, error)
    PresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
    Close(ctx context.Context) error
}
```
