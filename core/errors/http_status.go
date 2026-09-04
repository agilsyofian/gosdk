package errors

import (
	"encoding/json"
	"net/http"
)

// HTTPStatus represents a validated HTTP status code.
//
// The code field is intentionally unexported. This makes HTTPStatus a
// "sealed enum": other packages CANNOT construct arbitrary instances
// (e.g. errors.HTTPStatus{code: 999} will not compile outside this
// package). The only way to obtain a valid HTTPStatus is through one
// of the Status* variables defined below.
//
// Important note: errors.HTTPStatus{} (an empty literal, no named
// fields) can still be constructed from outside the package and
// produces the zero value (code 0) — this is the one loophole Go's
// type system cannot fully close. It is handled via IsZero() and
// validated at the entry point (newAppError), following a fail-closed
// principle: an empty/invalid status falls back to 500 instead of
// being propagated as-is.
type HTTPStatus struct {
	code int
}

// Int returns the raw int value of this status.
func (s HTTPStatus) Int() int {
	return s.code
}

// String returns the standard reason phrase, e.g. "Bad Request".
func (s HTTPStatus) String() string {
	if text := http.StatusText(s.code); text != "" {
		return text
	}
	return "Unknown Status"
}

// IsZero reports whether the status has never been set (zero value),
// e.g. when a caller creates an empty HTTPStatus{} instead of using a
// predefined constant.
func (s HTTPStatus) IsZero() bool {
	return s.code == 0
}

// IsSuccess reports whether the status falls in the 2xx range.
func (s HTTPStatus) IsSuccess() bool {
	return s.code >= 200 && s.code < 300
}

// IsClientError reports whether the status falls in the 4xx range.
func (s HTTPStatus) IsClientError() bool {
	return s.code >= 400 && s.code < 500
}

// IsServerError reports whether the status falls in the 5xx range.
func (s HTTPStatus) IsServerError() bool {
	return s.code >= 500 && s.code < 600
}

// MarshalJSON serializes HTTPStatus as a plain number in JSON, e.g.
// "status": 400 — not as an object like {"code":400}. This keeps the
// API response contract identical to when Status was a plain int.
func (s HTTPStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.code)
}

// Predefined HTTPStatus values — the only sanctioned source of values
// used across the VELLO / oneframework-sdk codebase. Add new entries
// here when a new status is needed; never construct HTTPStatus anywhere
// else.
var (
	// --- 2xx Success ---
	StatusOK = HTTPStatus{200}

	// --- 4xx Client ExpectedError ---
	StatusBadRequest         = HTTPStatus{400}
	StatusUnauthorized       = HTTPStatus{401}
	StatusPaymentRequired    = HTTPStatus{402}
	StatusForbidden          = HTTPStatus{403}
	StatusNotFound           = HTTPStatus{404}
	StatusMethodNotAllowed   = HTTPStatus{405}
	StatusNotAcceptable      = HTTPStatus{406}
	StatusRequestTimeout     = HTTPStatus{408}
	StatusConflict           = HTTPStatus{409}
	StatusGone               = HTTPStatus{410}
	StatusPreconditionFailed = HTTPStatus{412}
	StatusPayloadTooLarge    = HTTPStatus{413}
	StatusLocked             = HTTPStatus{423}
	StatusTooManyRequests    = HTTPStatus{429}

	// --- 5xx server ExpectedError ---
	StatusInternalServerError = HTTPStatus{500}
	StatusNotImplemented      = HTTPStatus{501}
	StatusBadGateway          = HTTPStatus{502}
	StatusServiceUnavailable  = HTTPStatus{503}
	StatusGatewayTimeout      = HTTPStatus{504}
)
