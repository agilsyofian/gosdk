package metadata

import (
	"context"
	"strings"
	"sync"
)

type contextKey struct{}

// Metadata represents standard contextual attributes and dynamic header baggage
// extracted across transport layers (HTTP, gRPC, etc.).
type Metadata struct {
	mu sync.RWMutex

	// Standard universal distributed tracing & identity attributes
	Authorization string `json:"authorization,omitempty"`
	TraceID       string `json:"trace_id,omitempty"`
	SpanID        string `json:"span_id,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	ClientIP      string `json:"client_ip,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`

	// Dynamic attributes (case-insensitive baggage for company-specific headers)
	values map[string]string
}

// New creates an empty Metadata container with initialized baggage storage.
func New() *Metadata {
	return &Metadata{
		values: make(map[string]string),
	}
}

// Get retrieves a metadata value by key (case-insensitive).
// It searches standard known fields first, then falls back to dynamic baggage.
func (m *Metadata) Get(key string) string {
	if m == nil {
		return ""
	}

	normKey := strings.ToLower(strings.TrimSpace(key))
	switch normKey {
	case "authorization", "auth":
		if m.Authorization != "" {
			return m.Authorization
		}
	case "trace_id", "traceid", "x-trace-id", "traceparent":
		if m.TraceID != "" {
			return m.TraceID
		}
	case "span_id", "spanid", "x-span-id":
		if m.SpanID != "" {
			return m.SpanID
		}
	case "session_id", "sessionid", "x-session-id":
		if m.SessionID != "" {
			return m.SessionID
		}
	case "request_id", "requestid", "x-request-id", "x-correlation-id":
		if m.RequestID != "" {
			return m.RequestID
		}
	case "user_id", "userid", "x-user-id":
		if m.UserID != "" {
			return m.UserID
		}
	case "client_ip", "clientip", "x-forwarded-for", "x-real-ip":
		if m.ClientIP != "" {
			return m.ClientIP
		}
	case "timestamp", "x-timestamp":
		if m.Timestamp != "" {
			return m.Timestamp
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.values == nil {
		return ""
	}
	return m.values[normKey]
}

// Set stores a dynamic metadata value under a case-insensitive key.
// If the key corresponds to a standard field, it updates the standard field as well.
func (m *Metadata) Set(key, value string) {
	if m == nil {
		return
	}

	normKey := strings.ToLower(strings.TrimSpace(key))
	if normKey == "" {
		return
	}

	switch normKey {
	case "authorization", "auth":
		m.Authorization = value
	case "trace_id", "traceid", "x-trace-id":
		m.TraceID = value
	case "span_id", "spanid", "x-span-id":
		m.SpanID = value
	case "session_id", "sessionid", "x-session-id":
		m.SessionID = value
	case "request_id", "requestid", "x-request-id", "x-correlation-id":
		m.RequestID = value
	case "user_id", "userid", "x-user-id":
		m.UserID = value
	case "client_ip", "clientip":
		m.ClientIP = value
	case "timestamp", "x-timestamp":
		m.Timestamp = value
	}

	m.mu.Lock()
	if m.values == nil {
		m.values = make(map[string]string)
	}
	m.values[normKey] = value
	m.mu.Unlock()
}

// GetAll returns a snapshot of all standard and dynamic metadata attributes.
func (m *Metadata) GetAll() map[string]string {
	if m == nil {
		return make(map[string]string)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.values)+8)
	for k, v := range m.values {
		result[k] = v
	}

	if m.Authorization != "" {
		result["authorization"] = m.Authorization
	}
	if m.TraceID != "" {
		result["trace_id"] = m.TraceID
	}
	if m.SpanID != "" {
		result["span_id"] = m.SpanID
	}
	if m.SessionID != "" {
		result["session_id"] = m.SessionID
	}
	if m.RequestID != "" {
		result["request_id"] = m.RequestID
	}
	if m.UserID != "" {
		result["user_id"] = m.UserID
	}
	if m.ClientIP != "" {
		result["client_ip"] = m.ClientIP
	}
	if m.Timestamp != "" {
		result["timestamp"] = m.Timestamp
	}

	return result
}

// Clone returns an independent deep copy of Metadata.
func (m *Metadata) Clone() *Metadata {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := &Metadata{
		Authorization: m.Authorization,
		TraceID:       m.TraceID,
		SpanID:        m.SpanID,
		SessionID:     m.SessionID,
		RequestID:     m.RequestID,
		UserID:        m.UserID,
		ClientIP:      m.ClientIP,
		Timestamp:     m.Timestamp,
		values:        make(map[string]string, len(m.values)),
	}

	for k, v := range m.values {
		clone.values[k] = v
	}

	return clone
}

// WithContext returns a new context carrying the given Metadata.
func WithContext(ctx context.Context, m *Metadata) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, m)
}

// FromContext extracts Metadata from ctx. Returns (nil, false) if not present.
func FromContext(ctx context.Context) (*Metadata, bool) {
	if ctx == nil {
		return nil, false
	}
	m, ok := ctx.Value(contextKey{}).(*Metadata)
	return m, ok && m != nil
}

// Value extracts a single metadata value by key directly from ctx.
// Returns an empty string if metadata or the key is absent.
func Value(ctx context.Context, key string) string {
	if m, ok := FromContext(ctx); ok {
		return m.Get(key)
	}
	return ""
}
