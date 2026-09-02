# Project Principles

These principles apply to the **current gosdk source tree**, not to an idealized future architecture.

## 1. Source truth over aspirational documentation

The implementation in the repository is authoritative. If an older document describes a package or feature that is absent from the source, treat that feature as unimplemented.

## 2. Stability over novelty

This SDK is consumed as reusable infrastructure. Prefer small, compatible changes over broad rewrites. Avoid changing exported names, signatures, semantics, or error behavior without an explicit reason.

## 3. Module independence

The repository is a Go workspace with multiple `go.mod` files. Keep leaf modules independently buildable. A dependency should be introduced at the narrowest module that actually needs it.

## 4. Idiomatic Go first

Prefer standard Go interfaces, explicit control flow, small types, `context.Context`, wrapped errors, and composition. Do not imitate application frameworks where ordinary Go is sufficient.

## 5. Fail safe in security-sensitive code

Authentication and authorization failures must never silently become successful operations. Security state should be explicit and inspectable.

## 6. Observability is a shared concern

Existing logger/tracing abstractions should be reused by I/O-heavy modules. Do not create ad-hoc logging or tracing protocols when the repository already provides one.

## 7. Public APIs are contracts

Exported types and functions are compatibility surfaces. New APIs should be minimal and composable. Avoid exposing implementation details that callers do not need.

## 8. Tests are part of the design

A behavior change should normally be accompanied by a test that demonstrates the intended contract. Concurrency, retry, timeout, circuit-breaker, security, and storage behavior deserve particular attention.

## 9. Dependency restraint

Every third-party dependency increases the maintenance and security surface of a reusable SDK. Reuse dependencies already present in the relevant module before adding another library.

## 10. Documentation must describe reality

Keep `AI_AGENT.md`, `.instructions/*`, `xdocs/*`, examples, and package comments consistent with the source. Do not document future integrations as current functionality.
