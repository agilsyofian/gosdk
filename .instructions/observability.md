# Observability

All observability primitives are housed within the single unified module `github.com/agilsyofian/gosdk/core/observability` and exposed through clean subpackages:
- `core/observability/logger`
- `core/observability/tracing`
- `core/observability/health`
- `core/observability/metrics`
- `core/observability/audit`

## 1. Logging

The implemented logging abstraction is `core/observability/logger.Logger`, with a zerolog-backed implementation.

Use the logger interface rather than depending directly on zerolog in packages that only need logging behavior. Preserve context propagation and the existing `LogPayload` structure.

Never log:

- passwords;
- API keys;
- bearer/JWT tokens;
- private keys;
- raw session secrets;
- unnecessary full request/response bodies containing sensitive data.

`FilterSensitiveData` exists to sanitize dynamic payloads; reuse it where appropriate.

## 2. Tracing

`observability/tracing` uses OpenTelemetry and provides context metadata helpers for app name, trace ID, and session ID.

Transport and I/O integrations should preserve incoming context and trace metadata rather than creating disconnected contexts.

## 3. Health

`observability/health` provides a registry of named checks and aggregates their status. Checks should honor context cancellation and should not perform unbounded work.

## 4. Metrics

`observability/metrics` currently contains package documentation only. It is **not** a functional metrics abstraction in the current source. Do not instruct agents to call a metrics API that does not exist.

## 5. Audit

`observability/audit` currently contains package documentation only. It is **not** a functional audit sink/API in the current source. Do not invent an audit interface as if it already existed.

If audit support is added later, it should be introduced deliberately with its own API and tests.

## 6. Transport observability

Existing HTTP/gRPC clients and servers already provide logger/tracer middleware/interceptors. Extend those mechanisms rather than creating parallel observability hooks.

## 7. Error context

When wrapping errors, add context that helps identify the operation without leaking secrets. Preserve the underlying error for inspection.

## 8. What not to do

Do not add noisy logs to tight loops, log the same error at every layer, or claim metrics/audit coverage that the current implementation does not provide.
