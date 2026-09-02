package crypto

import "fmt"

// Metadata carries the request context that must be bound into AES-GCM
// as Additional Authenticated Data, per STEERFILE.md §9. The backend
// MUST reconstruct exactly the same Metadata (from trusted request
// context, not from the envelope body) before decryption; any mismatch
// causes AES-GCM authentication to fail.
//
// TenantID is optional and omitted from the AAD line when empty, so
// single-tenant deployments don't need to set it.
type Metadata struct {
	Version     string // protocol version, e.g. ProtocolVersion1
	Method      string // HTTP method, e.g. "POST"
	Path        string // API path, e.g. "/api/v1/payment"
	KeyID       string // key_id used for wrapping
	RequestID   string // request_id from the envelope
	ContentType string // e.g. "application/json"
	TenantID    string // optional tenant/client identifier
}

// buildAAD renders Metadata into the exact deterministic byte sequence
// used as AES-GCM Additional Authenticated Data. The format is a
// canonical newline-separated "key=value" list with a fixed field order
// so that frontend and backend never disagree on serialization, per
// STEERFILE.md §9 and §21.
//
// This ordering and format is part of the versioned protocol: changing
// it requires a new ProtocolVersion, per STEERFILE.md §25.
func buildAAD(m Metadata) []byte {
	s := fmt.Sprintf(
		"v=%s\nmethod=%s\npath=%s\nkey_id=%s\nrequest_id=%s\ncontent_type=%s",
		m.Version, m.Method, m.Path, m.KeyID, m.RequestID, m.ContentType,
	)
	if m.TenantID != "" {
		s += "\ntenant_id=" + m.TenantID
	}
	return []byte(s)
}
