module github.com/agilsyofian/gosdk/transport/client/http

go 1.26.8

require (
	github.com/agilsyofian/gosdk/core/resilience v0.0.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/sony/gobreaker/v2 v2.4.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
)

replace github.com/agilsyofian/gosdk/core/resilience => ./../../../core/resilience
