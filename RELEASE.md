# Go SDK Release Guide

This document outlines the workflow, standards, and procedures for tagging and releasing modules within the Go SDK monorepo (`github.com/agilsyofian/gosdk`).

---

## 1. Go Modules Monorepo Tagging Architecture

The `gosdk` repository is structured as a multi-module monorepo containing 30 independent submodules. Following the official [Go Modules VCS Versioning Specification](https://go.dev/ref/mod#vcs-version), release tags for modules located in subdirectories **must use the format**:

```text
<module-subpath>/v<major>.<minor>.<patch>
```

### Tagging Examples:
| Module | Git Tag Format | Consumer `go get` Command |
|---|---|---|
| `core/observability` | `core/observability/v1.0.0` | `go get github.com/agilsyofian/gosdk/core/observability@v1.0.0` |
| `storage/objectstorage/s3` | `storage/objectstorage/s3/v1.0.0` | `go get github.com/agilsyofian/gosdk/storage/objectstorage/s3@v1.0.0` |
| `storage/database/postgres` | `storage/database/postgres/v1.0.0` | `go get github.com/agilsyofian/gosdk/storage/database/postgres@v1.0.0` |
| `storage/cache/redis` | `storage/cache/redis/v1.0.0` | `go get github.com/agilsyofian/gosdk/storage/cache/redis@v1.0.0` |
| `transport/server/http` | `transport/server/http/v1.0.0` | `go get github.com/agilsyofian/gosdk/transport/server/http@v1.0.0` |

---

## 2. Semantic Versioning (SemVer) Standards

Every release version adheres to the `vMAJOR.MINOR.PATCH` convention:
- **MAJOR (`v2.0.0`)**: Breaking or incompatible changes to the public API or interfaces.
- **MINOR (`v1.1.0`)**: Backwards-compatible new features (e.g., new storage provider, new middleware).
- **PATCH (`v1.0.1`)**: Backwards-compatible bug fixes, security patches, or performance improvements without altering public function signatures.

---

## 3. Pre-Release Validation Checklist

Before creating tags or pushing releases to the remote repository, ensure all checks pass:

1. **Format Code**:
   ```bash
   make fmt
   # Or auto-format with:
   make fmt-fix
   ```
2. **Static Analysis & Vet**:
   ```bash
   make vet
   ```
3. **Unit Tests (100% Pass)**:
   ```bash
   make test
   ```
4. **Tidy Dependencies**:
   ```bash
   make tidy
   ```

---

## 4. Using the `release.sh` Release Script

An automated tagging script is provided at the root of the repository: [release.sh](release.sh).

### Interactive Mode:
Run without arguments to open an interactive terminal menu:
```bash
./release.sh
```

### Single Module Release (CLI Mode):
```bash
# Create local tag
./release.sh storage/objectstorage/s3 v1.0.0

# Create tag and push immediately to remote
./release.sh storage/objectstorage/s3 v1.0.0 -p

# Add an annotated tag message
./release.sh storage/objectstorage/s3 v1.0.0 -m "Release S3 & MinIO driver v1.0.0" -p
```

### Batch Release All Modules:
Useful for the initial repository release or simultaneous major/minor upgrades:
```bash
# Dry-run simulation first
./release.sh --dry-run --all v1.0.0

# Create tags for all 30 modules
./release.sh --all v1.0.0 -m "Initial Release v1.0.0 for all Go SDK modules"

# Create and push all tags to origin
./release.sh --all v1.0.0 --push
```

### List Modules & Current Tags:
```bash
./release.sh --list
```

---

## 5. Usage in Downstream Services

Backend microservices consuming Go SDK modules can install them directly using standard `go get`:

```bash
# Install S3/MinIO object storage provider
go get github.com/agilsyofian/gosdk/storage/objectstorage/s3@v1.0.0

# Install PostgreSQL database driver
go get github.com/agilsyofian/gosdk/storage/database/postgres@v1.0.0

# Install HTTP server transport
go get github.com/agilsyofian/gosdk/transport/server/http@v1.0.0
```

Then run `go mod tidy` in the consuming project.
