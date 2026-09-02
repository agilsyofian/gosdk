# Steering File --- Frontend ↔ Backend Field Encryption

## 1. Purpose

This steering file defines the implementation contract for encrypting
selected sensitive values from a frontend request before sending them to
the backend.

The design goal is:

-   Keep existing request-body JSON structures unchanged.
-   Encrypt only explicitly selected fields.
-   Use hybrid encryption.
-   Use AES-256-GCM for field values.
-   Use RSA-OAEP-SHA256 to wrap the per-request AES key.
-   Publish only backend public encryption keys.
-   Carry encryption metadata in HTTP headers.
-   Keep replay protection as a separate middleware responsibility.
-   Make the protocol deterministic and interoperable between frontend,
    Postman, and the Go backend SDK.

This specification is intended for AI-assisted code generation.
Implementations MUST follow this protocol unless a deliberate versioned
protocol change is introduced.

------------------------------------------------------------------------

## 2. Security Model

The system uses hybrid encryption:

``` text
Frontend
    |
    | Generate random AES-256 key
    |
    +--> AES-256-GCM --> encrypted sensitive fields
    |
    +--> RSA-OAEP-SHA256 using Backend PUBLIC KEY
              |
              +--> encrypted AES key
    |
    +--> HTTP request
              |
              v
Backend
    |
    +--> Backend PRIVATE KEY
    |       |
    |       +--> decrypt AES key
    |
    +--> AES-256-GCM
            |
            +--> decrypt sensitive fields
```

The frontend MUST NEVER receive or contain the backend private
encryption key.

The backend private key MUST NOT be exposed through JWKS.

HTTPS remains mandatory. Application-level encryption is an additional
security layer and does not replace TLS.

------------------------------------------------------------------------

## 3. Key Types

### 3.1 Backend Encryption Key

Backend encryption keys are asymmetric RSA key pairs.

The backend owns:

``` text
Private Key
Public Key
```

The frontend receives only:

``` text
Public Key
```

The public key is identified by `kid`.

Recommended JWK:

``` json
{
  "use": "enc",
  "kty": "RSA",
  "alg": "RSA-OAEP-256",
  "kid": "encryption-2026-08",
  "n": "...",
  "e": "AQAB",
  "x5c": []
}
```

### 3.2 `use`

For encryption keys:

``` text
use = enc
```

JWT signing keys are separate and use:

``` text
use = sig
```

Encryption keys and JWT signing keys MUST NOT be mixed in the same
logical key configuration.

### 3.3 `kid`

`kid` is the key identifier.

Example:

``` text
encryption-2026-08
encryption-2026-01
```

The active key is selected using its `kid`.

Multiple keys MUST be supported to allow key rotation.

------------------------------------------------------------------------

## 4. JWKS

JWKS means JSON Web Key Set.

The backend MAY expose its public encryption keys through a JWKS-style
endpoint, for example:

``` text
GET /.well-known/jwks.json
```

The response format is:

``` json
{
  "keys": [
    {
      "use": "enc",
      "kty": "RSA",
      "alg": "RSA-OAEP-256",
      "kid": "encryption-2026-08",
      "n": "...",
      "e": "AQAB",
      "x5c": []
    }
  ]
}
```

Only public JWK parameters are allowed in the response.

Private parameters such as RSA `d`, `p`, `q`, etc. MUST NEVER be
returned.

------------------------------------------------------------------------

## 5. Configuration

For YAML/Viper configuration:

``` yaml
encryption:
  jwks:
    keys:
      - use: enc
        kty: RSA
        alg: RSA-OAEP-256
        kid: encryption-2026-08
        n: "..."
        e: "AQAB"
        x5c: []

      - use: enc
        kty: RSA
        alg: RSA-OAEP-256
        kid: encryption-2026-01
        n: "..."
        e: "AQAB"
        x5c: []
```

Expected Go model:

``` go
type Key struct {
    Use string   `json:"use" mapstructure:"use" validate:"required"`
    Kty string   `json:"kty" mapstructure:"kty" validate:"required"`
    Alg string   `json:"alg" mapstructure:"alg" validate:"required"`
    Kid string   `json:"kid" mapstructure:"kid" validate:"required"`
    N   string   `json:"n" mapstructure:"n" validate:"required"`
    E   string   `json:"e" mapstructure:"e" validate:"required"`
    X5c []string `json:"x5c" mapstructure:"x5c"`
}

type Jwks struct {
    Keys []Key `json:"keys" mapstructure:"keys" validate:"required"`
}
```

Viper SHOULD be used for loading the YAML configuration.

------------------------------------------------------------------------

## 6. Encryption Scope

The implementation MUST support encrypting selected request-body fields
without changing the request schema.

Example original request:

``` json
{
  "name": "Budi",
  "nik": "1234567890123456",
  "phone": "08123456789",
  "address": "Jakarta"
}
```

Configured encrypted fields:

``` text
nik
phone
```

Result:

``` json
{
  "name": "Budi",
  "nik": "BASE64URL_ENCRYPTED_VALUE",
  "phone": "BASE64URL_ENCRYPTED_VALUE",
  "address": "Jakarta"
}
```

The JSON field types SHOULD remain unchanged where the original field is
a string.

The protocol MUST NOT require replacing:

``` json
"nik": "..."
```

with:

``` json
"nik": {
  "iv": "...",
  "ciphertext": "...",
  "tag": "..."
}
```

No encryption metadata object is required inside the request body.

------------------------------------------------------------------------

## 7. Per-Request AES Key

For every HTTP request containing encrypted fields:

1.  Generate a fresh cryptographically secure random AES-256 key.
2.  Use that AES key for the encrypted fields in that request.
3.  Never reuse the AES key across requests.

AES-256 key size:

``` text
32 bytes
```

The AES key MUST be generated using a cryptographically secure random
number generator.

------------------------------------------------------------------------

## 8. Per-Field IV

Each encrypted field MUST use a fresh random AES-GCM IV.

Recommended IV size:

``` text
96 bits
12 bytes
```

Example:

``` text
Request #1

AES Key
  |
  +--> NIK   -> IV1 -> AES-GCM
  |
  +--> Phone -> IV2 -> AES-GCM
```

`IV1` and `IV2` MUST NOT be reused with the same AES key.

It is acceptable for the same field to receive a different IV on every
request.

The IV is embedded into the encrypted value so the request body
structure does not need to change.

------------------------------------------------------------------------

## 9. AES-256-GCM

Sensitive field plaintext MUST be encrypted using:

``` text
AES-256-GCM
```

Recommended authentication tag size:

``` text
128 bits
16 bytes
```

Each encrypted field produces:

``` text
IV
Ciphertext
Authentication Tag
```

No unauthenticated encryption mode is allowed.

------------------------------------------------------------------------

## 10. Encrypted Value Format

The encrypted field value MUST be encoded as:

``` text
Base64URL(
    IV
    || Ciphertext
    || Authentication Tag
    || Nonce
    || Timestamp
)
```

The binary layout is:

``` text
+----------+-------------+----------+------------+-----------+
| IV 12 B  | Ciphertext  | Tag 16 B | Nonce 16 B | Time 8 B  |
+----------+-------------+----------+------------+-----------+
```

Where:

-   `IV` = random 96-bit AES-GCM IV.
-   `Ciphertext` = AES-GCM encrypted plaintext.
-   `Tag` = 128-bit AES-GCM authentication tag.
-   `Nonce` = request-level replay-protection nonce.
-   `Timestamp` = Unix timestamp in seconds, encoded as unsigned 64-bit
    big-endian.

The complete binary value is Base64URL encoded without padding.

Example:

``` json
{
  "nik": "QmFzZTY0VVJMRW5jcnlwdGVkVmFsdWU",
  "phone": "QW5vdGhlckJhc2U2NFVSTFZhbHVl"
}
```

Base64/Base64URL is an encoding mechanism only. It MUST NOT be treated
as encryption.

------------------------------------------------------------------------

## 11. Request-Level Nonce and Timestamp

Every encrypted HTTP request MUST generate:

``` text
X-Nonce
X-Timestamp
```

The nonce is generated once per HTTP request and reused in the encrypted
values belonging to that request.

Recommended nonce size:

``` text
16 bytes / 128 bits
```

The timestamp is generated once per HTTP request.

Recommended representation:

``` text
Unix timestamp in seconds
```

Example:

``` http
X-Nonce: <Base64URL random 16-byte nonce>
X-Timestamp: 1788172800
```

The nonce and timestamp are also embedded in every encrypted value
according to the encrypted-value binary format.

------------------------------------------------------------------------

## 12. HTTP Headers

The encryption protocol SHOULD use:

``` http
Content-Type: application/json
X-Encryption-Version: 1
X-Encryption-Kid: encryption-2026-08
X-Encryption-Key: <Base64URL RSA-OAEP-SHA256 encrypted AES key>
X-Nonce: <Base64URL request nonce>
X-Timestamp: <Unix timestamp seconds>
```

Meaning:

  -----------------------------------------------------------------------
  Header                              Purpose
  ----------------------------------- -----------------------------------
  `X-Encryption-Version`              Protocol version

  `X-Encryption-Kid`                  Backend public encryption key
                                      identifier

  `X-Encryption-Key`                  AES-256 key wrapped using backend
                                      RSA public key

  `X-Nonce`                           Request-level replay-protection
                                      nonce

  `X-Timestamp`                       Request creation timestamp
  -----------------------------------------------------------------------

The header nonce and timestamp MUST correspond to the values embedded in
the encrypted field.

------------------------------------------------------------------------

## 13. RSA-OAEP Key Wrapping

The AES-256 key MUST be wrapped using the backend RSA public key.

Algorithm:

``` text
RSA-OAEP
Hash: SHA-256
MGF1: SHA-256
```

Equivalent JWK algorithm identifier:

``` text
RSA-OAEP-256
```

The encrypted AES key is Base64URL encoded and placed in:

``` http
X-Encryption-Key
```

RSA encryption MUST happen once per HTTP request, not once per encrypted
field.

------------------------------------------------------------------------

## 14. Complete Frontend Encryption Flow

Frontend implementation SHOULD follow this exact sequence:

1.  Select the active backend encryption public key using `kid`.
2.  Generate a fresh random AES-256 key.
3.  Generate a fresh random request nonce.
4.  Generate the current Unix timestamp.
5.  Read the request body.
6.  Identify configured sensitive fields.
7.  For every sensitive field:
    1.  Generate a fresh 96-bit AES-GCM IV.

    2.  Encrypt the field using AES-256-GCM.

    3.  Obtain the ciphertext and 128-bit authentication tag.

    4.  Build:

        ``` text
        IV || Ciphertext || Tag || Nonce || Timestamp
        ```

    5.  Base64URL encode the result.

    6.  Replace only the field value.
8.  Encrypt the request AES key using backend RSA-OAEP-SHA256 public
    key.
9.  Put encryption metadata into HTTP headers.
10. Send the request over HTTPS.
11. Remove unnecessary plaintext sensitive values from temporary
    storage.

------------------------------------------------------------------------

## 15. Backend Decryption Flow

The backend encryption middleware/adapter SHOULD:

1.  Read encryption headers.

2.  Validate the protocol version.

3.  Resolve the RSA private key using `X-Encryption-Kid`.

4.  Read `X-Encryption-Key`.

5.  Base64URL decode the encrypted AES key.

6.  RSA-OAEP-SHA256 decrypt the AES key.

7.  Read configured encrypted fields.

8.  Base64URL decode each encrypted field.

9.  Parse:

    ``` text
    IV
    Ciphertext
    Tag
    Nonce
    Timestamp
    ```

10. Validate that embedded nonce matches `X-Nonce`.

11. Validate that embedded timestamp matches `X-Timestamp`.

12. AES-256-GCM decrypt each field.

13. Replace encrypted field values with plaintext.

14. Pass the restored request body to the existing application layer.

The existing application request structs SHOULD remain unchanged.

------------------------------------------------------------------------

## 16. Replay Protection

Replay protection is a separate responsibility from cryptographic
decryption.

The replay middleware SHOULD use:

``` text
X-Nonce
X-Timestamp
```

The timestamp provides request freshness.

The nonce provides request uniqueness.

Recommended default timestamp window:

``` text
5 minutes
```

Example:

``` text
server time = 10:00:00

valid:
09:55:00 <= request timestamp <= 10:05:00

outside window:
reject
```

The exact clock-skew policy SHOULD be configurable.

The nonce MUST be checked against a replay store.

The replay store SHOULD perform an atomic check-and-consume operation.

Example Redis semantics:

``` text
SET <nonce> <marker> NX EX 300
```

If the nonce already exists, the request is considered a replay and MUST
be rejected.

Replay protection MUST be implemented outside the low-level `Decrypt()`
function.

------------------------------------------------------------------------

## 17. Cryptographic Separation of Responsibility

Low-level decryptor responsibility:

``` text
RSA-OAEP-SHA256
AES-256-GCM
authentication verification
```

It SHOULD NOT own:

``` text
HTTP
Redis
nonce persistence
timestamp policy
replay detection
request routing
```

Replay protection responsibility:

``` text
timestamp validation
nonce validation
nonce uniqueness
replay-store interaction
```

Recommended abstraction:

``` go
type ReplayProtector interface {
    Validate(
        ctx context.Context,
        nonce string,
        timestamp int64,
    ) error
}
```

The exact interface MAY differ according to the SDK architecture.

------------------------------------------------------------------------

## 18. Key Rotation

Multiple encryption keys MUST be supported.

Example:

``` yaml
encryption:
  jwks:
    keys:
      - kid: encryption-2026-08
        use: enc
        kty: RSA
        alg: RSA-OAEP-256
        n: "..."
        e: "AQAB"

      - kid: encryption-2026-01
        use: enc
        kty: RSA
        alg: RSA-OAEP-256
        n: "..."
        e: "AQAB"
```

Frontend selects the active key.

Backend keeps the corresponding private key available for decrypting
requests encrypted with still-valid key IDs.

Key rotation MUST NOT require changing the request-body schema.

------------------------------------------------------------------------

## 19. Postman Testing Contract

Postman Pre-request Script SHOULD simulate the frontend protocol.

Configuration:

``` text
ENCRYPTION_KID
ENCRYPTION_PUBLIC_JWK
```

Sensitive fields:

``` javascript
const ENCRYPT_FIELDS = [
    "nik",
    "phone"
];
```

Postman MUST:

-   Generate AES-256 key per request.

-   Generate request nonce per request.

-   Generate request timestamp per request.

-   Generate a different IV per encrypted field.

-   Encrypt each configured field with AES-256-GCM.

-   Build:

    ``` text
    IV || Ciphertext || Tag || Nonce || Timestamp
    ```

-   Base64URL encode the result.

-   RSA-OAEP-SHA256 encrypt the AES key.

-   Set the required HTTP headers.

-   Preserve the original request-body structure.

------------------------------------------------------------------------

## 20. Frontend Browser Implementation

Production frontend implementations MUST use:

-   Web Crypto API, or
-   an established, audited cryptographic library.

Frontend MUST NOT implement cryptographic primitives manually.

The frontend MUST NOT embed backend private keys.

The frontend public-key retrieval mechanism SHOULD support key rotation.

------------------------------------------------------------------------

## 21. Security Requirements

The implementation MUST:

-   Use cryptographically secure random generation.
-   Use AES-256-GCM.
-   Use 96-bit random IVs.
-   Never reuse an IV with the same AES key.
-   Generate a new AES key for every request.
-   Generate a new request nonce for every request.
-   Use RSA-OAEP-SHA256 for AES-key wrapping.
-   Keep backend private keys server-side.
-   Never expose private JWK parameters through JWKS.
-   Use HTTPS.
-   Validate `kid`.
-   Validate encryption algorithm.
-   Validate key usage (`use=enc`).
-   Validate nonce consistency.
-   Validate timestamp freshness.
-   Reject replayed nonces.
-   Avoid logging plaintext sensitive values.
-   Avoid logging encryption keys, private keys, decrypted payloads, or
    complete encrypted headers.

------------------------------------------------------------------------

## 22. Error Handling

The encryption layer SHOULD distinguish failures such as:

``` text
invalid encryption version
unknown kid
invalid encryption key
invalid nonce
invalid timestamp
expired request
replayed request
invalid encrypted value
invalid IV
invalid authentication tag
decryption failed
```

Authentication/decryption failures SHOULD NOT expose detailed
cryptographic internals to external clients.

Operational logs MAY contain a safe error classification and request
correlation ID, but MUST NOT contain secret key material or plaintext
sensitive values.

------------------------------------------------------------------------

## 23. Versioning

The protocol is explicitly versioned:

``` http
X-Encryption-Version: 1
```

Any incompatible binary-layout, header, algorithm, or semantic change
MUST introduce a new protocol version.

Version 1 encrypted value format:

``` text
Base64URL(
    IV
    || Ciphertext
    || Tag
    || Nonce
    || Timestamp
)
```

------------------------------------------------------------------------

## 24. Reference Example

Original request:

``` json
{
  "name": "Budi",
  "nik": "1234567890123456",
  "phone": "08123456789",
  "address": "Jakarta"
}
```

After encryption:

``` json
{
  "name": "Budi",
  "nik": "BASE64URL(IV1 + CIPHERTEXT1 + TAG1 + NONCE + TIMESTAMP)",
  "phone": "BASE64URL(IV2 + CIPHERTEXT2 + TAG2 + NONCE + TIMESTAMP)",
  "address": "Jakarta"
}
```

Headers:

``` http
X-Encryption-Version: 1
X-Encryption-Kid: encryption-2026-08
X-Encryption-Key: BASE64URL(RSA-OAEP-SHA256(AES-256-KEY))
X-Nonce: BASE64URL(NONCE)
X-Timestamp: 1788172800
```

Backend processing:

``` text
HTTP Request
    |
    +--> Replay Middleware
    |      |
    |      +--> timestamp validation
    |      +--> nonce validation
    |
    +--> Encryption Middleware
           |
           +--> resolve kid
           +--> RSA-OAEP decrypt AES key
           +--> decode encrypted fields
           +--> validate nonce/timestamp
           +--> AES-GCM decrypt fields
           |
           v
      Existing Request Struct
```

------------------------------------------------------------------------

## 25. Important Design Constraint

The protocol intentionally does NOT wrap the entire request body in an
opaque encryption envelope.

Only selected sensitive values are encrypted.

This is required to preserve compatibility with existing request structs
and application handlers.

The desired model is:

``` text
Existing Request Struct
        |
        +--> sensitive string -> encrypted string
        |
        +--> normal string    -> plaintext string
```

No additional JSON wrapper is required around individual encrypted
fields.

------------------------------------------------------------------------

## 26. Implementation Priority

When generating code from this steering file, follow this priority:

1.  Preserve protocol compatibility.
2.  Preserve existing request-body structures.
3.  Preserve cryptographic correctness.
4.  Preserve key-rotation support.
5.  Preserve replay-protection separation.
6.  Avoid unnecessary API changes.
7.  Keep crypto implementation isolated from HTTP/application concerns.
8.  Add tests for interoperability and failure cases.

The implementation MUST NOT simplify the protocol by reusing IVs,
reusing AES keys across requests, exposing private keys, or moving
encryption metadata into the request-body schema.
