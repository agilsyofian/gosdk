# Coding Style

## 1. Formatting and tooling

Run `gofmt` on changed Go files. Use the repository's available `Makefile` targets and standard Go tooling when applicable. Do not introduce formatting conventions that fight `gofmt`.

## 2. Exported identifiers

Every exported type, function, method, constant, or variable should have a Go doc comment beginning with its identifier, unless the existing package intentionally uses a different established pattern. Keep comments factual and aligned with behavior.

## 3. Naming

Use idiomatic Go names: `ID`, `URL`, `HTTP`, `JWT`, `JWKS`, `API`, `TTL`, etc. Avoid redundant names such as `GetX` when the package/type context already makes the operation obvious.

## 4. Errors

- Return errors instead of panicking for expected runtime failures.
- Wrap errors when adding meaningful context.
- Use the repository's `errors` package where the caller needs structured application error information.
- Preserve sentinel/type identity when callers may use `errors.Is`/`errors.As`.
- Do not compare error strings as the primary control mechanism.

## 5. Context

Use `context.Context` as the first argument for I/O, network, storage, authentication lookups, and operations that can be cancelled. Do not store a context in a long-lived struct unless the API explicitly requires it.

## 6. Configuration

The current source uses ordinary config structs and constructors; there is no generic project-wide configuration package. Follow each existing module's constructor/config shape instead of introducing a new configuration framework.

## 7. Zero values and structs

Keep useful zero values where practical. Preserve existing field semantics and ordering when modifying public structs. Do not add hidden global mutable state.

## 8. Concurrency

Protect shared mutable state with `sync` primitives or other appropriate mechanisms. Prefer ownership and message passing where it simplifies correctness. Tests for concurrent state should use the race detector when relevant.

## 9. Imports

Keep imports grouped and formatted by `gofmt`. Do not use blank imports unless they are explicitly required for registration/side effects.

## 10. Generics

Use generics only when they materially improve the API or remove repetitive, type-unsafe code. Do not retrofit generics into stable APIs without a clear benefit.

## 11. Logging

Use `observability/logger.Logger` where the module already depends on the SDK logger. Never add ad-hoc `fmt.Println`/`log.Printf` for runtime framework logging.

## 12. Security-sensitive code

Avoid logging credentials, tokens, passwords, session secrets, or raw sensitive request data. Reuse existing filtering and security abstractions.
