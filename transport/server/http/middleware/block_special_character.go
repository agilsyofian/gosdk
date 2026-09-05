package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	appError "github.com/agilsyofian/gosdk/core/errors"
	"github.com/gin-gonic/gin"
)

var (
	defaultCharsRegex = regexp.MustCompile(`^[a-zA-Z0-9., '/_-]*$`)
	base64Regex       = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
	hexRegex          = regexp.MustCompile(`^[0-9a-fA-F]+$`)
)

type ValidationResult struct {
	IsValid bool
	Path    string
	Value   string
}

type specialCharMatcher struct {
	allowedRegex *regexp.Regexp
}

func newSpecialCharMatcher(customRegex string) *specialCharMatcher {
	regexPattern := strings.TrimSpace(customRegex)
	if regexPattern == "" {
		regexPattern = strings.TrimSpace(os.Getenv("ALLOWED_CHARS_REGEX"))
	}

	if regexPattern != "" {
		if re, err := regexp.Compile(regexPattern); err == nil {
			return &specialCharMatcher{allowedRegex: re}
		}
	}

	return &specialCharMatcher{allowedRegex: defaultCharsRegex}
}

func (m *specialCharMatcher) isForbidden(input string) bool {
	if m.allowedRegex.MatchString(input) {
		return false
	}
	return !checkValidAES(input)
}

func checkValidAES(input string) bool {
	return isValidAESBase64(input) || isValidAESHex(input)
}

func isValidAESBase64(input string) bool {
	// Minimum AES block size is 16 bytes, which encodes to 24 chars in Base64
	if len(input) < 24 || len(input)%4 != 0 {
		return false
	}
	return base64Regex.MatchString(input)
}

func isValidAESHex(input string) bool {
	if len(input) == 0 || len(input)%32 != 0 {
		return false
	}
	return hexRegex.MatchString(input)
}

func buildPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func buildArrayPath(parent string, index int) string {
	if parent == "" {
		return fmt.Sprintf("[%d]", index)
	}
	return fmt.Sprintf("%s[%d]", parent, index)
}

func checkJSONValue(v any, path string, matcher *specialCharMatcher) ValidationResult {
	switch value := v.(type) {
	case string:
		if matcher.isForbidden(value) {
			return ValidationResult{
				IsValid: false,
				Path:    path,
				Value:   value,
			}
		}
	case map[string]any:
		for key, val := range value {
			res := checkJSONValue(val, buildPath(path, key), matcher)
			if !res.IsValid {
				return res
			}
		}
	case []any:
		for i, val := range value {
			res := checkJSONValue(val, buildArrayPath(path, i), matcher)
			if !res.IsValid {
				return res
			}
		}
	}
	return ValidationResult{IsValid: true}
}

func isJSONPayload(c *gin.Context, body []byte) bool {
	ct := c.ContentType()
	if strings.Contains(ct, "application/json") {
		return true
	}
	if ct == "" {
		trimmed := bytes.TrimSpace(body)
		return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
	}
	return false
}

// BlockJSONSpecialChars returns a Gin middleware that inspects JSON request bodies
// and blocks dangerous characters (XSS, SQLi, command injection) with HTTP 400 Bad Request.
// Legitimate AES ciphertext strings (Hex / Base64) are exempted.
func BlockJSONSpecialChars(appName string, customRegex ...string) gin.HandlerFunc {
	regexPattern := ""
	if len(customRegex) > 0 {
		regexPattern = customRegex[0]
	}
	matcher := newSpecialCharMatcher(regexPattern)

	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			appErr := appError.BadRequest(err, appName, "400", "Failed to read request body")
			_ = c.Error(appErr)
			c.Status(http.StatusBadRequest)
			c.Abort()
			return
		}

		// Restore the body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		if len(body) == 0 || !isJSONPayload(c, body) {
			c.Next()
			return
		}

		var data any
		if err := json.Unmarshal(body, &data); err != nil {
			appErr := appError.BadRequest(err, appName, "400", "Invalid JSON payload")
			_ = c.Error(appErr)
			c.Status(http.StatusBadRequest)
			c.Abort()
			return
		}

		result := checkJSONValue(data, "", matcher)
		if !result.IsValid {
			msg := fmt.Sprintf("Special characters detected in field '%s' with value: %s", result.Path, result.Value)
			appErr := appError.New(
				errors.New(msg),
				appError.StatusBadRequest,
				appName,
				"400",
				msg,
				map[string]string{
					"field": result.Path,
					"value": result.Value,
				},
			)
			_ = c.Error(appErr)
			c.Status(http.StatusBadRequest)
			c.Abort()
			return
		}

		c.Next()
	}
}
