package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// AppError is a structured application error carrying HTTP status,
// classification metadata, and an optional wrapped cause.
type AppError struct {
	Status       HTTPStatus `json:"status"`
	Source       string     `json:"source,omitempty"`
	Code         string     `json:"code,omitempty"`
	Message      string     `json:"message,omitempty"`
	MessageError string     `json:"messageError,omitempty"`
	Location     string     `json:"function,omitempty"`
	Cause        error      `json:"cause,omitempty"`
	Result       any        `json:"result,omitempty"`
}

// Default values used by Unknown/InternalServerError.
const (
	DefaultSource  = "server"
	DefaultCode    = "0019"
	DefaultMessage = "Internal server ExpectedError"
)

// GeneralSource, GeneralCode, and GeneralMessage remain mutable
// package-level vars so callers can reconfigure them; they default to
// the constants above.
var (
	GeneralSource  = DefaultSource
	GeneralCode    = DefaultCode
	GeneralMessage = DefaultMessage
)

// skipFrames lists path fragments excluded when locating the caller's
// origin frame for error reporting.
var skipFrames = []string{
	"/runtime/",
	"/errors/",
	"/middleware/",
	"/github.com/gin-gonic/gin/",
}

// framesToCapture bounds the stack walk to avoid unbounded allocation.
const framesToCapture = 32

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d:%s:%s:%s:%s", e.Status.Int(), e.Source, e.Code, e.Message, e.Location)
	if e.Cause != nil {
		fmt.Fprintf(&sb, ":%v", e.Cause)
	}
	return sb.String()
}

// Is enables errors.Is comparisons based on Code, so callers can check
// sentinel-style AppErrors without relying on pointer identity.
func (e *AppError) Is(target error) bool {
	var t *AppError
	if !errors.As(target, &t) {
		return false
	}
	return e.Code != "" && e.Code == t.Code
}

// --- Constructors -----------------------------------------------------

// ExpectedError creates an application error with status 200 (commonly used for
// business-logic errors that still return HTTP 200).
func ExpectedError(err error, source, code, message string) error {
	return newAppError(err, StatusOK, source, code, message, nil)
}

func BadRequest(err error, source, code, message string) error {
	return newAppError(err, StatusBadRequest, source, code, message, nil)
}

func Unauthorized(err error, source, code, message string) error {
	return newAppError(err, StatusUnauthorized, source, code, message, nil)
}

func PaymentRequired(err error, source, code, message string) error {
	return newAppError(err, StatusPaymentRequired, source, code, message, nil)
}

func Forbidden(err error, source, code, message string) error {
	return newAppError(err, StatusForbidden, source, code, message, nil)
}

func NotFound(err error, source, code, message string) error {
	return newAppError(err, StatusNotFound, source, code, message, nil)
}

func MethodNotAllowed(err error, source, code, message string) error {
	return newAppError(err, StatusMethodNotAllowed, source, code, message, nil)
}

func NotAcceptable(err error, source, code, message string) error {
	return newAppError(err, StatusNotAcceptable, source, code, message, nil)
}

func RequestTimeout(err error, source, code, message string) error {
	return newAppError(err, StatusRequestTimeout, source, code, message, nil)
}

func Conflict(err error, source, code, message string) error {
	return newAppError(err, StatusConflict, source, code, message, nil)
}

func Gone(err error, source, code, message string) error {
	return newAppError(err, StatusGone, source, code, message, nil)
}

func PreconditionFailed(err error, source, code, message string) error {
	return newAppError(err, StatusPreconditionFailed, source, code, message, nil)
}

func PayloadTooLarge(err error, source, code, message string) error {
	return newAppError(err, StatusPayloadTooLarge, source, code, message, nil)
}

func Locked(err error, source, code, message string) error {
	return newAppError(err, StatusLocked, source, code, message, nil)
}

func TooManyRequests(err error, source, code, message string) error {
	return newAppError(err, StatusTooManyRequests, source, code, message, nil)
}

func InternalServerError(err error, source, code, message string) error {
	return newAppError(err, StatusInternalServerError, source, code, message, nil)
}

func NotImplemented(err error, source, code, message string) error {
	return newAppError(err, StatusNotImplemented, source, code, message, nil)
}

func BadGateway(err error, source, code, message string) error {
	return newAppError(err, StatusBadGateway, source, code, message, nil)
}

func ServiceUnavailable(err error, source, code, message string) error {
	return newAppError(err, StatusServiceUnavailable, GeneralSource, GeneralCode, GeneralMessage, nil)
}

func GatewayTimeout(err error, source, code, message string) error {
	return newAppError(err, StatusGatewayTimeout, GeneralSource, GeneralCode, GeneralMessage, nil)
}

func Unknown(err error) error {
	return newAppError(err, StatusInternalServerError, GeneralSource, GeneralCode, GeneralMessage, nil)
}

// String builds an AppError
func String(message string) error {
	return errors.New(message)
}

// New builds an AppError with full control over every field, including
// a wrapped cause. Status remains an HTTPStatus, not an int.
func New(err error, status HTTPStatus, source, code, message string, result any) error {
	return newAppError(err, status, source, code, message, result)
}

// Wrap normalizes err into an *AppError. If err already is one, its
// fields are preserved but Location is recomputed for the current call
// site. Non-AppErrors become a generic InternalServerError. Wrap(nil) returns
// nil.
func Wrap(err error) error {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return newAppError(
			appErr.Cause,
			appErr.Status,
			appErr.Source,
			appErr.Code,
			appErr.Message,
			appErr.Result,
		)
	}

	return Unknown(err)
}

// newAppError is the single internal constructor. If status is the zero
// value (e.g. a caller slipped past the sealed enum via an empty
// HTTPStatus{}), it falls back to 500 instead of propagating an invalid
// status — a fail-closed default.
func newAppError(err error, status HTTPStatus, source, code, message string, result any) *AppError {
	if status.IsZero() {
		status = StatusInternalServerError
	}
	return &AppError{
		Status:       status,
		Source:       source,
		Code:         code,
		Message:      message,
		MessageError: err.Error(),
		Location:     findLocation(),
		Cause:        err,
		Result:       result,
	}
}

// --- Location resolution ------------------------------------------------

// findLocation walks the call stack to find the first frame outside of
// runtime/errors/middleware/gin internals, returning "file:line". Skip
// depth is fixed at 3 to account for: runtime.Callers itself,
// findLocation, and newAppError.
func findLocation() string {
	pcs := make([]uintptr, framesToCapture)
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return "unknown"
	}

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !shouldSkip(frame.File) {
			return fmt.Sprintf("%s:%d", frame.File, frame.Line)
		}
		if !more {
			break
		}
	}

	return "unknown"
}

func shouldSkip(file string) bool {
	for _, s := range skipFrames {
		if strings.Contains(file, s) {
			return true
		}
	}
	return false
}
