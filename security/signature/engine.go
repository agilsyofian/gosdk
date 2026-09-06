package signature

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMissingSignature = errors.New("signature: missing signature header")
	ErrMissingTimestamp = errors.New("signature: missing timestamp header")
	ErrTimestampExpired = errors.New("signature: timestamp expired or clock skew too large")
	ErrInvalidTimestamp = errors.New("signature: invalid timestamp format")
	ErrInvalidSignature = errors.New("signature: invalid signature")
)

// Engine executes request signature verification and response signing.
type Engine struct {
	cfg Config
}

// NewEngine constructs a signature Engine.
func NewEngine(provider SecretProvider, opts ...Option) *Engine {
	cfg := DefaultConfig(provider)
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Engine{cfg: cfg}
}

// Config returns the engine's configuration.
func (e *Engine) Config() Config {
	return e.cfg
}

// VerifyRequest verifies the incoming HTTP request signature and returns the resolved keyID and secret.
func (e *Engine) VerifyRequest(ctx context.Context, req *http.Request, body []byte) (string, []byte, error) {
	sigHeader := strings.TrimSpace(req.Header.Get(e.cfg.SignatureHeader))
	if sigHeader == "" {
		return "", nil, fmt.Errorf("%w: %s", ErrMissingSignature, e.cfg.SignatureHeader)
	}

	tsHeader := strings.TrimSpace(req.Header.Get(e.cfg.TimestampHeader))
	if tsHeader == "" {
		return "", nil, fmt.Errorf("%w: %s", ErrMissingTimestamp, e.cfg.TimestampHeader)
	}

	if err := e.validateTimestamp(tsHeader); err != nil {
		return "", nil, err
	}

	keyID := strings.TrimSpace(req.Header.Get(e.cfg.KeyIDHeader))
	secret, err := e.cfg.SecretProvider.GetSecret(ctx, keyID)
	if err != nil {
		return keyID, nil, err
	}

	stringToSign, err := e.cfg.Canonicalizer.BuildStringToSign(req, body, tsHeader)
	if err != nil {
		return keyID, secret, err
	}

	if !Verify(e.cfg.Algorithm, []byte(stringToSign), sigHeader, secret, e.cfg.Encoding) {
		return keyID, secret, ErrInvalidSignature
	}

	return keyID, secret, nil
}

// SignResponse generates the signature and timestamp for an outgoing HTTP response.
func (e *Engine) SignResponse(statusCode int, body []byte, secret []byte) (string, string, error) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	stringToSign, err := e.cfg.ResponseCanonicalizer.BuildResponseStringToSign(statusCode, body, timestamp)
	if err != nil {
		return "", "", err
	}

	sig, err := Sign(e.cfg.Algorithm, []byte(stringToSign), secret, e.cfg.Encoding)
	if err != nil {
		return "", "", err
	}

	return sig, timestamp, nil
}

func (e *Engine) validateTimestamp(tsStr string) error {
	if e.cfg.MaxClockDrift <= 0 {
		return nil
	}

	parsedTime, err := parseTimestamp(tsStr)
	if err != nil {
		return err
	}

	diff := time.Since(parsedTime)
	if diff < 0 {
		diff = -diff
	}

	if diff > e.cfg.MaxClockDrift {
		return ErrTimestampExpired
	}

	return nil
}

func parseTimestamp(val string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z0700",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, val); err == nil {
			return t, nil
		}
	}

	// Try numeric epoch timestamp (seconds or milliseconds)
	if num, err := strconv.ParseInt(val, 10, 64); err == nil {
		if num > 1e11 { // Milliseconds
			return time.UnixMilli(num), nil
		}
		return time.Unix(num, 0), nil
	}

	return time.Time{}, ErrInvalidTimestamp
}
