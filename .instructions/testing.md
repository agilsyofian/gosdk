# Testing

## 1. Test the changed contract

For every behavior change, add or update tests that demonstrate the intended behavior. Pure documentation changes do not require code tests.

Recommended minimums:

| Change | Test expectation |
|---|---|
| New exported behavior | success path + meaningful failure path |
| Bug fix | regression test reproducing the old failure |
| Security behavior | positive + negative/fail-closed cases |
| Stateful/concurrent behavior | concurrency/race coverage where relevant |
| Transport middleware/interceptor | request/response or RPC behavior without real external infrastructure |

## 2. Unit tests should be isolated

Do not make ordinary unit tests depend on a live Redis, database, Kafka/RabbitMQ broker, or external HTTP service.

Use existing interfaces and fakes. For transport code, prefer in-memory test servers/connections such as `httptest` or gRPC `bufconn` where appropriate.

## 3. Table-driven tests

Use table-driven tests when multiple cases exercise the same behavior. Keep each case explicit and use `t.Run` for readable failures.

## 4. Test module boundaries through interfaces

Prefer small hand-written fakes over introducing a new mocking framework. Reuse whatever mocking approach the affected module already uses.

## 5. Security tests

Security changes should include adversarial cases such as:

- invalid credentials;
- expired sessions/tokens;
- malformed/tampered tokens;
- unknown users/keys;
- missing security context;
- authorization denial;
- dependency lookup failure that must fail closed.

Do not remove negative tests to make a change pass.

## 6. Assertions

Prefer stable error types/sentinels and semantic fields over full error-string comparisons. Use `errors.Is`/`errors.As` where appropriate.

## 7. Coverage

Do not lower coverage in the changed package without a reason. Security, resilience, session, and stateful storage paths deserve strong edge-case coverage.

## 8. Fuzzing

Consider Go fuzz tests for parsers handling untrusted input, especially JWT parsing, credential parsing, and request decoding. The key invariant is that malformed input must not panic or bypass validation.

## 9. Race detector

Run `go test -race` for changes involving shared state, especially memory sessions, resilience policies, caches, or transport concurrency.

## 10. Workspace validation

Because the repository contains multiple modules, validate the affected module(s) and the workspace/example module when the change crosses module boundaries. Do not assume `go test ./...` from the repository root covers every independent module.

## 11. Do not hide failures

Never use `t.Skip()` to suppress a failing test without explicitly documenting why the test is intentionally excluded. Fix the defect or flag the issue.
