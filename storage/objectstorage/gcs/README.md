# storage/objectstorage/gcs

Google Cloud Storage (GCS) backend implementing the [`objectstorage.Storage`](../storage.go) interface.

Powered by the official [`cloud.google.com/go/storage`](https://cloud.google.com/go/storage) SDK.

---

## Features

| Feature | Detail |
|---|---|
| Cloud-Native Auth | Native support for **GKE Workload Identity** & **Application Default Credentials (ADC)** without static JSON keys |
| Service Account Auth | Optional direct service account key via JSON string, base64 string, or file path |
| Observability | Distributed OpenTelemetry client spans (`gcs.upload`, `gcs.download`, `gcs.delete`, etc.) |
| Health Check | Implements `health.Check` for Kubernetes `/actuator/readiness` probes |
| Presigned URLs | Secure, signed GET URLs for direct client downloads |
| Streaming | Native `io.Reader` and `io.ReadCloser` (prevents high memory consumption & OOM) |
| Pure Go | Decoupled from DI frameworks, easily wired into Fx or standard `main()` |

---

## Installation

```go
// go.mod
require github.com/agilsyofian/gosdk/storage/objectstorage/gcs <version>
```

---

## Quick Start (Pure Go)

### 1. Using Workload Identity / ADC (Recommended for GKE / Cloud Run)

```go
package main

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "log"

    "github.com/agilsyofian/gosdk/core/observability/logger"
    "github.com/agilsyofian/gosdk/storage/objectstorage/gcs"
)

func main() {
    ctx := context.Background()
    l := logger.NewZerologLogger(logger.Config{Output: "stdout"})

    // Leave credentials empty to use Workload Identity / ADC automatically
    cfg := gcs.Config{
        Bucket: "my-gcp-bucket",
    }

    storage, err := gcs.NewGCSStorage(ctx, l, cfg)
    if err != nil {
        log.Fatalf("failed to init gcs storage: %v", err)
    }
    defer storage.Close(ctx)

    // Upload
    content := []byte("Hello GCS!")
    err = storage.Upload(ctx, "reports/2026-summary.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
    if err != nil {
        log.Fatalf("upload error: %v", err)
    }

    // Download stream
    rc, err := storage.Download(ctx, "reports/2026-summary.txt")
    if err != nil {
        log.Fatalf("download error: %v", err)
    }
    defer rc.Close()

    data, _ := io.ReadAll(rc)
    fmt.Println("Downloaded:", string(data))
}
```

### 2. Using Service Account File / JSON

```go
cfg := gcs.Config{
    Bucket:          "my-gcp-bucket",
    CredentialsFile: "/etc/secrets/gcp-sa-key.json", // or CredentialsJSON: "{...}"
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
    "github.com/agilsyofian/gosdk/storage/objectstorage/gcs"
    "go.uber.org/fx"
)

var GCSModule = fx.Module("gcs",
    fx.Provide(
        func(lc fx.Lifecycle, l logger.Logger, hr health.Registry) (objectstorage.Storage, error) {
            ctx := context.Background()
            cfg := gcs.Config{
                Bucket: "app-documents-bucket",
            }
            store, err := gcs.NewGCSStorage(ctx, l, cfg, hr)
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
    Bucket          string // GCS Bucket Name (required)
    CredentialsJSON string // Raw or base64 JSON Service Account key (optional)
    CredentialsFile string // Path to Service Account JSON key file (optional)
    ProjectID       string // GCP Project ID (optional)
}
```
