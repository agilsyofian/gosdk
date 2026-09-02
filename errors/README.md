# Errors Package

The `errors` package provides the SDK's standardized application error contract. It carries an operationally controlled HTTP status, source/code metadata, a message, optional result data, and an optional wrapped cause.

## HTTP status contract

HTTP status is intentionally **not an arbitrary `int`** in the public constructors. Use the SDK's `Status*` constants from `errors/http_status.go`.

Supported statuses are:

- `StatusOK`, `StatusCreated`, `StatusAccepted`, `StatusNoContent`
- `StatusMovedPermanently`, `StatusFound`, `StatusNotModified`
- `StatusBadRequest`, `StatusUnauthorized`, `StatusPaymentRequired`, `StatusForbidden`, `StatusNotFound`, `StatusMethodNotAllowed`, `StatusNotAcceptable`, `StatusRequestTimeout`, `StatusConflict`, `StatusGone`, `StatusPreconditionFailed`, `StatusPayloadTooLarge`, `StatusUnprocessableEntity`, `StatusLocked`, `StatusTooManyRequests`
- `StatusInternalServerError`, `StatusNotImplemented`, `StatusBadGateway`, `StatusServiceUnavailable`, `StatusGatewayTimeout`

This allow-list is an operational contract for consistent HTTP mapping and observability (for example, Dynatrace). It must not be expanded casually.

`HTTPStatus.IsValid()` can be used to validate a value. Constructors that accept an explicit status also validate it and return `ErrInvalidHTTPStatus` for unsupported values.

> The explicit-status constructors use a sealed `HTTPStatusOption`, so `net/http` status constants and arbitrary `int` values cannot be passed directly. Callers must use one of the SDK `Status*` constants. An explicit conversion such as `errors.HTTPStatus(418)` is possible in Go, so runtime allow-list validation remains the final enforcement layer.

## Creating errors

Prefer the semantic constructors when they match the error:

```go
return errors.ValidationError("user", "0001", "user ID is invalid")
return errors.Unauthorized("auth", "0002", "authentication required")
return errors.Forbidden("auth", "0003", "access denied")
return errors.NotFound("user", "0004", "user not found")
return errors.Conflict("user", "0005", "user already exists")
return errors.TooManyRequests("api", "0006", "rate limit exceeded")
```

When an explicit supported status is required, use an SDK status constant:

```go
return errors.ErrorWithHTTPStatus(
    "payment",
    "0007",
    "payment is required",
    errors.StatusPaymentRequired,
)
```

For full control, `New` uses the same controlled status contract:

```go
return errors.New(
    err,
    errors.StatusServiceUnavailable,
    "payment",
    "0008",
    "payment service unavailable",
    nil,
)
```

If a status is constructed dynamically, validate it before using it or handle the constructor error:

```go
err := errors.ErrorWithHTTPStatus("api", "0009", "unsupported status", errors.HTTPStatus(418))
if errors.Is(err, errors.ErrInvalidHTTPStatus) {
    // Reject the invalid application status configuration.
}
```

## Wrapping errors

`Wrap()` normalizes an error into `*AppError` while preserving an existing `AppError` contract or converting an unknown error into a generic internal error.

```go
if err != nil {
    return errors.Wrap(err)
}
```

`Wrap(nil)` returns `nil`.

## Error inspection

Use standard Go `errors.Is` and `errors.As`:

```go
var appErr *errors.AppError
if errors.As(err, &appErr) {
    // appErr.Status, appErr.Code, appErr.Source, appErr.Message, ...
}
```

`AppError.Is` compares application errors by `Code`, while `AppError.Unwrap` exposes the underlying cause to the standard error chain.

## Compatibility notes

The existing semantic constructors (`ValidationError`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `TooManyRequests`, `GeneralError`, `InternalError`, `Error`, and `ErrorWithResult`) keep their existing behavior.

The explicit-status APIs now accept only the sealed `HTTPStatusOption` contract and enforce the SDK allow-list. This is intentionally a stricter API contract: callers should migrate from arbitrary integer status variables to the corresponding SDK `Status*` constant.
