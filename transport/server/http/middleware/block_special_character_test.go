package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupSpecialCharsRouter(appName string, customRegex ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(
		middleware.Error(logger.Nop(), appName, true),
		middleware.BlockJSONSpecialChars(appName, customRegex...),
	)
	r.POST("/submit", func(c *gin.Context) {
		var body map[string]any
		_ = c.ShouldBindJSON(&body)
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": body})
	})
	return r
}

func TestBlockJSONSpecialChars_AllowedPayloads(t *testing.T) {
	r := setupSpecialCharsRouter("test-app")

	tests := []struct {
		name    string
		payload string
	}{
		{"Normal text", `{"name": "John Doe", "age": 30}`},
		{"Alphanumeric and allowed symbols", `{"path": "/users/john_doe-123.txt", "notes": "hello, world 'test'"}`},
		{"Empty string and spaces", `{"empty": "", "spaces": "   "}`},
		{"Valid AES Base64", `{"cipher": "uN7zX8jKl9mNoPqRsTuVwA=="}`},
		{"Valid AES Hex", `{"hex": "4a6f686e20446f652053656372657421"}`}, // 32 hex chars
		{"Nested objects", `{"user": {"profile": {"city": "Jakarta", "country": "Indonesia"}}}`},
		{"Arrays of allowed items", `{"tags": ["admin", "staff", "user-1"]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Expected OK for: %s, got: %s", tt.payload, w.Body.String())
		})
	}
}

func TestBlockJSONSpecialChars_BlockedPayloads(t *testing.T) {
	r := setupSpecialCharsRouter("test-app")

	tests := []struct {
		name          string
		payload       string
		expectedField string
	}{
		{"Basic HTML script tag", `{"input": "<script>alert(1)</script>"}`, "input"},
		{"HTML with event handler", `{"input": "<img src=x onerror=alert(1)>"}`, "input"},
		{"SQL injection semicolon", `{"query": "SELECT * FROM users; DROP TABLE users;"}`, "query"},
		{"Shell pipe", `{"cmd": "user | ls -la"}`, "cmd"},
		{"Special chars with HTML", `{"comment": "test < > & \""}`, "comment"},
		{"Nested field injection", `{"data": {"nested": {"field": "bad<script>"}}}`, "data.nested.field"},
		{"Array element injection", `{"items": ["good", "evil<script>"]}`, "items[1]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var resp dto.BaseResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.Equal(t, "test-app", resp.Source)
			assert.Equal(t, "400", resp.Code)
			assert.Contains(t, resp.Message, "Special characters detected in field")
			assert.Contains(t, resp.Message, tt.expectedField)
		})
	}
}

func TestBlockJSONSpecialChars_CustomRegex(t *testing.T) {
	// Custom regex that strictly only allows digits
	r := setupSpecialCharsRouter("test-app", `^[0-9]+$`)

	// Digits only -> OK
	reqOK := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewBufferString(`{"pin": "123456"}`))
	reqOK.Header.Set("Content-Type", "application/json")
	wOK := httptest.NewRecorder()
	r.ServeHTTP(wOK, reqOK)
	assert.Equal(t, http.StatusOK, wOK.Code)

	// Letters -> blocked
	reqBad := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewBufferString(`{"pin": "abc123"}`))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, wBad.Code)
}

func TestBlockJSONSpecialChars_EnvCustomRegex(t *testing.T) {
	t.Setenv("ALLOWED_CHARS_REGEX", `^[a-zA-Z]+$`)
	r := setupSpecialCharsRouter("test-app")

	reqOK := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewBufferString(`{"name": "AlphabetOnly"}`))
	reqOK.Header.Set("Content-Type", "application/json")
	wOK := httptest.NewRecorder()
	r.ServeHTTP(wOK, reqOK)
	assert.Equal(t, http.StatusOK, wOK.Code)

	reqBad := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewBufferString(`{"name": "Alpha123"}`))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, wBad.Code)
}

func TestBlockJSONSpecialChars_InvalidJSON(t *testing.T) {
	r := setupSpecialCharsRouter("test-app")

	req := httptest.NewRequest(http.MethodPost, "/submit", bytes.NewBufferString(`{not-valid-json}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
