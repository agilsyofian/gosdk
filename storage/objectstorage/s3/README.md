# storage/objectstorage/s3

Cloud-agnostic S3 and MinIO object storage backend implementing the [`objectstorage.Storage`](../storage.go) interface.

Powered by [`minio-go/v7`](https://github.com/minio/minio-go), compatible with:
- **Amazon Web Services (AWS S3)**
- **MinIO** (Self-hosted / Local development)
- **Google Cloud Storage** (via XML API interoperability)
- **Ceph / DigitalOcean Spaces / Cloudflare R2**

---

## Features

| Feature | Detail |
|---|---|
| Streaming | Native `io.Reader` and `io.ReadCloser` (prevents high memory consumption & OOM) |
| Observability | Distributed OpenTelemetry client spans (`s3.upload`, `s3.download`, etc.) |
| Health Check | Implements `health.Check` for Kubernetes `/actuator/readiness` probes |
| IAM Safety | `AutoCreateBucket: false` by default, avoiding `AccessDenied` on restricted IAM roles |
| Presigned URLs | Secure, time-limited direct client upload/download URLs |
| Pure Go | Decoupled from DI frameworks, easily wired into Fx or standard `main()` |

---

## Installation

```go
// go.mod
require github.com/agilsyofian/gosdk/storage/objectstorage/s3 <version>
```

---

## Quick Start (Pure Go)

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "log"
    "time"

    "github.com/agilsyofian/gosdk/core/observability/logger"
    "github.com/agilsyofian/gosdk/storage/objectstorage/s3"
)

func main() {
    ctx := context.Background()
    l := logger.NewZerologLogger(logger.Config{Output: "stdout"})

    cfg := s3.Config{
        Endpoint:  "s3.amazonaws.com",
        AccessKey: "YOUR_ACCESS_KEY",
        SecretKey: "YOUR_SECRET_KEY",
        Bucket:    "my-production-bucket",
        Region:    "ap-southeast-1",
        UseSSL:    true,
    }

    storage, err := s3.NewS3Storage(l, cfg)
    if err != nil {
        log.Fatalf("failed to init s3 storage: %v", err)
    }
    defer storage.Close(ctx)

    // Upload
    content := []byte("Hello Object Storage!")
    err = storage.Upload(ctx, "documents/test.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
    if err != nil {
        log.Fatalf("upload error: %v", err)
    }

    // Presigned URL
    url, err := storage.PresignedURL(ctx, "documents/test.txt", 15*time.Minute)
    fmt.Println("Presigned URL:", url)

    // Download stream
    rc, err := storage.Download(ctx, "documents/test.txt")
    if err != nil {
        log.Fatalf("download error: %v", err)
    }
    defer rc.Close()
    
    data, _ := io.ReadAll(rc)
    fmt.Println("Downloaded:", string(data))
}
```

---

## Uber Fx Integration (Application Layer)

```go
package bootstrap

import (
    "context"

    "github.com/agilsyofian/gosdk/core/observability/health"
    "github.com/agilsyofian/gosdk/core/observability/logger"
    "github.com/agilsyofian/gosdk/storage/objectstorage"
    "github.com/agilsyofian/gosdk/storage/objectstorage/s3"
    "go.uber.org/fx"
)

var ObjectStorageModule = fx.Module("objectstorage",
    fx.Provide(
        func(lc fx.Lifecycle, l logger.Logger, hr health.Registry) (objectstorage.Storage, error) {
            cfg := s3.Config{
                Endpoint:  "s3.ap-southeast-1.amazonaws.com",
                AccessKey: "AKIA...",
                SecretKey: "...",
                Bucket:    "app-bucket",
                Region:    "ap-southeast-1",
                UseSSL:    true,
            }
            store, err := s3.NewS3Storage(l, cfg, hr)
            if err != nil {
                return nil, err
            }
            lc.Append(fx.Hook{
                OnStop: func(ctx context.Context) error {
                    return store.Close(ctx)
                },
            })
            return store, nil
        },
    ),
)
```

---

## Configuration Reference

```go
type Config struct {
    Endpoint         string        // S3 host (e.g. "s3.amazonaws.com" or "localhost:9000")
    AccessKey        string        // IAM Access Key ID
    SecretKey        string        // IAM Secret Access Key
    Bucket           string        // Target Bucket Name
    UseSSL           bool          // Use HTTPS (default: true)
    Region           string        // AWS Region (e.g. "ap-southeast-1")
    AutoCreateBucket bool          // Auto create bucket if not exists (default: false)
    PartSize         uint64        // Multipart upload chunk size (default: 5MB)
    RequestTimeout   time.Duration // Default: 30s
}
```
