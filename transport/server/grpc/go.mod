module github.com/agilsyofian/gosdk/transport/server/grpc

go 1.26.8

require (
	github.com/agilsyofian/gosdk/core/metadata v0.0.1
	github.com/agilsyofian/gosdk/core/observability v0.0.1
	github.com/agilsyofian/gosdk/security v0.0.1
	github.com/agilsyofian/gosdk/transport/idempotency v0.0.1
	github.com/google/uuid v1.6.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
	go.uber.org/fx v1.24.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
)

replace (
	github.com/agilsyofian/gosdk/core/metadata => ./../../../core/metadata
	github.com/agilsyofian/gosdk/core/observability => ./../../../core/observability
	github.com/agilsyofian/gosdk/security => ./../../../security
	github.com/agilsyofian/gosdk/transport/idempotency => ./../../idempotency
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260831171406-18b4a7587f8a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260831171406-18b4a7587f8a // indirect
)
