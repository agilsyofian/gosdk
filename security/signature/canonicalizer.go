package signature

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
)

// Canonicalizer builds the string-to-sign from an incoming HTTP request.
type Canonicalizer interface {
	BuildStringToSign(req *http.Request, body []byte, timestamp string) (string, error)
}

// ResponseCanonicalizer builds the string-to-sign for an outgoing HTTP response.
type ResponseCanonicalizer interface {
	BuildResponseStringToSign(statusCode int, body []byte, timestamp string) (string, error)
}

// CustomCanonicalizerFunc adapts a function to the Canonicalizer interface.
type CustomCanonicalizerFunc func(req *http.Request, body []byte, timestamp string) (string, error)

func (f CustomCanonicalizerFunc) BuildStringToSign(req *http.Request, body []byte, timestamp string) (string, error) {
	return f(req, body, timestamp)
}

// CustomResponseCanonicalizerFunc adapts a function to the ResponseCanonicalizer interface.
type CustomResponseCanonicalizerFunc func(statusCode int, body []byte, timestamp string) (string, error)

func (f CustomResponseCanonicalizerFunc) BuildResponseStringToSign(statusCode int, body []byte, timestamp string) (string, error) {
	return f(statusCode, body, timestamp)
}

type defaultCanonicalizer struct{}

// NewDefaultCanonicalizer returns standard canonicalizer: method + "\n" + path + "\n" + timestamp + "\n" + sha256(body).
func NewDefaultCanonicalizer() Canonicalizer {
	return &defaultCanonicalizer{}
}

func (d *defaultCanonicalizer) BuildStringToSign(req *http.Request, body []byte, timestamp string) (string, error) {
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])

	path := req.URL.Path
	if req.URL.RawQuery != "" {
		path = path + "?" + req.URL.RawQuery
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s", req.Method, path, timestamp, bodyHashHex), nil
}

type defaultResponseCanonicalizer struct{}

// NewDefaultResponseCanonicalizer returns standard response canonicalizer: statusCode + "\n" + timestamp + "\n" + sha256(body).
func NewDefaultResponseCanonicalizer() ResponseCanonicalizer {
	return &defaultResponseCanonicalizer{}
}

func (d *defaultResponseCanonicalizer) BuildResponseStringToSign(statusCode int, body []byte, timestamp string) (string, error) {
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])
	return fmt.Sprintf("%d\n%s\n%s", statusCode, timestamp, bodyHashHex), nil
}
