# API Guidelines

## 1. Follow the existing API shape

Public APIs should match neighboring packages. Current patterns include constructors such as `New`, `NewServer`, `NewManager`, and policy constructors returning a configured value.

Do not invent a new constructor/configuration convention unless the module genuinely requires it.

## 2. I/O methods use context

Public methods performing network, storage, broker, or authentication lookup operations should accept `context.Context`, normally as the first argument.

Examples already present include `Execute(ctx, ...)`, `Dial(ctx, ...)`, session `Create/Get/Delete/Refresh`, and authenticator `Authenticate(ctx, ...)`.

## 3. Compatibility

Prefer:

- adding new methods/types over changing existing signatures;
- adding optional behavior through existing option/config patterns;
- preserving exported struct fields;
- preserving stable error identity.

Removing or renaming an exported symbol is a breaking change and requires an explicit decision.

## 4. Interfaces at consumer seams

Keep interfaces small. Existing examples include `security.Authenticator`, `security.Authorizer`, `security.SessionManager`, `storage.Cache`, `storage.Database`, logger and health interfaces.

Do not expose a concrete dependency when consumers only need a small behavior surface.

## 5. Typed enums

For values with a closed semantic set, use a named type such as `security.CredentialType` or `security` algorithm/status types rather than unrelated magic strings throughout the API.

## 6. Errors are inspectable

Use structured errors when callers need status/source/code/result information. Preserve wrapping so callers can inspect the underlying cause.

## 7. Cancellation and timeout semantics

Do not silently replace a caller's context with `context.Background()`. Respect cancellation. Use the existing resilience timeout/retry policies where the package already exposes them.

## 8. Concurrency is part of the contract

Document or test concurrency expectations for stateful exported types. In-memory sessions and resilience policies must not introduce data races.

## 9. Avoid `any` unless required

`any` is acceptable where the API is inherently dynamic, such as error result payloads or generic request/response bodies. Do not use it to avoid designing a meaningful type.

## 10. Public API versioning

The workspace currently uses independent module versions. When changing a module's public API, inspect that module's `go.mod`, callers in `examples`, and existing tests before making the change.

## 11. Examples

When a new public API is non-obvious, update the appropriate `xexamples` or `xdocs` material. Do not add an example merely to satisfy a rule if the API is self-explanatory.

## 12. Export checklist

Before exporting a new symbol, ask:

- Is this behavior already available?
- Is the smallest useful abstraction being exposed?
- Does it need context?
- What is the error contract?
- Is it safe for concurrent use?
- Can it be added without breaking existing callers?
- Does the owning module actually need the dependency?
