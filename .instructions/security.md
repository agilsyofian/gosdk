# Security

## 1. Secrets

Never commit passwords, private keys, API keys, JWTs, session secrets, database credentials, or production endpoints. Tests must use generated or clearly fake values.

## 2. Transport security

Do not weaken TLS verification or certificate validation merely to make a test/example work. Any insecure transport behavior must be explicit, scoped, and justified by the existing API contract.

## 3. Input validation

Treat credentials, authorization data, JWT claims, headers, session identifiers, and external payloads as untrusted. Validate before using them in security decisions.

## 4. Deserialization safety

Do not deserialize untrusted data into unsafe executable structures. For JWT/JWKS and request payloads, reject malformed input and preserve algorithm/claim validation.

## 5. Authentication and authorization

Current security primitives include:

- API key authentication;
- Basic authentication;
- JWT authentication/JWKS retrieval;
- role/permission authorization;
- memory and Redis sessions;
- security context propagation.

Security decisions must fail closed. A missing principal, invalid credential, expired session/token, lookup error, or malformed security context must not grant access.

## 6. Logging and sensitive data

Never log passwords, raw API keys, bearer tokens, private keys, or session secrets. Reuse `core/observability/logger` filtering where available.

## 7. Dependencies

Prefer maintained dependencies already used by the relevant module. Adding a cryptographic/security dependency requires reviewing its trust model and maintenance status.

## 8. Replay and session safety

Preserve expiration/TTL semantics. Do not make a session or token effectively permanent by accidentally ignoring TTLs. Treat session identifiers and authorization credentials as secrets.

## 9. Build hygiene

Do not introduce network downloads, generated credentials, or hidden runtime callbacks into build/test tooling. Migration and scaffolding tools must not contain production secrets or destructive defaults.

## 10. Vulnerabilities

If a change reveals a security weakness, fix the underlying unsafe behavior where the task permits and document the security impact. Do not hide the issue by weakening a test.

## 11. Never do these things

- Never disable certificate/signature/claim verification just to pass a test.
- Never hardcode authentication bypasses.
- Never change authorization failures into success paths.
- Never exfiltrate data to an undocumented endpoint.
- Never add real credentials to examples or fixtures.
