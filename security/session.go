package security

import (
	"time"
)

// Session represents server-side, stateful user context. It is the
// source of truth for "who is this user, right now" once
// authentication has succeeded.
type Session struct {
	// ID is the unique session identifier (the "sid").
	ID string

	// Principal is the identity this session belongs to.
	Principal *Principal

	CreatedAt time.Time
	ExpiresAt time.Time

	// Attributes holds arbitrary, business-defined session data, e.g.
	// device_id, channel, branch_id, customer_type, login_method.
	//
	// Do not store sensitive secrets here unless there is a clear
	// security requirement to do so.
	Attributes map[string]any
}

// Expired reports whether the session has passed its ExpiresAt time as
// of now.
func (s *Session) Expired() bool {
	if s == nil {
		return true
	}
	return time.Now().After(s.ExpiresAt)
}

// Attribute retrieves a session attribute by key.
func (s *Session) Attribute(key string) (any, bool) {
	if s == nil || s.Attributes == nil {
		return nil, false
	}
	v, ok := s.Attributes[key]
	return v, ok
}
