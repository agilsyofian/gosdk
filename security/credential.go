package security

// CredentialType identifies the kind of authentication credential
// presented by the caller.
type CredentialType string

const (
	CredentialBearer CredentialType = "bearer"
	CredentialBasic  CredentialType = "basic"
	CredentialAPIKey CredentialType = "api_key"
	CredentialHMAC   CredentialType = "hmac"
	CredentialMTLS   CredentialType = "mtls"
	CredentialOAuth  CredentialType = "oauth"

	// CredentialSession identifies a credential that is already a
	// resolved session identifier (e.g. extracted from a session
	// cookie), as opposed to a credential that must first be validated
	// by an Authenticator before a session ID can be derived from it.
	CredentialSession CredentialType = "session"
)

// Credential is a transport-independent representation of whatever the
// caller presented to authenticate. Gin extracts it from HTTP headers
// or cookies; gRPC extracts it from metadata. Neither the Credential
// type nor anything that consumes it should know which transport it
// came from.
type Credential struct {
	// Type identifies which Authenticator should handle this
	// credential.
	Type CredentialType

	// Value holds the raw credential material, e.g. a JWT string, an
	// "Authorization: Basic <base64>" payload, an API key, a session
	// ID, or an HMAC signature.
	Value string

	// Identifier optionally carries a secondary value that some
	// credential types require alongside Value, e.g. an API key ID
	// paired with an API key secret, or an HMAC key ID paired with a
	// signature.
	Identifier string
}
