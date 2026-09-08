package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/rs/zerolog"
)

func TestFilterSensitiveData(t *testing.T) {
	filterParams := []string{"password", "token", "secret"}
	maxLength := 100 // Diperbesar agar email tidak terpotong
	mask := "****"   // Sesuai permintaan di issue: berubah menjadi ****

	tests := []struct {
		name      string
		input     any
		maxLength int
		expected  map[string]any
	}{
		{
			name: "Nested JSON with sensitive fields",
			input: map[string]any{
				"user": map[string]any{
					"username": "johndoe",
					"password": "supersecretpassword",
					"profile": map[string]any{
						"email": "john@example.com",
						"token": "abc123token",
					},
				},
				"public_data": "some value",
			},
			expected: map[string]any{
				"user": map[string]any{
					"username": "johndoe",
					"password": mask,
					"profile": map[string]any{
						"email": "john@example.com",
						"token": mask,
					},
				},
				"public_data": "some value",
			},
		},
		{
			name: "Sensitive field is an object",
			input: map[string]any{
				"secret": map[string]any{
					"key":           "value",
					"nested_secret": "hidden",
				},
			},
			expected: map[string]any{
				"secret": map[string]any{
					"key":           "value",
					"nested_secret": "hidden",
				},
			},
		},
		{
			name: "Sensitive field is an array",
			input: map[string]any{
				"token": []any{"token1", "token2"},
			},
			expected: map[string]any{
				"token": []any{"token1", "token2"},
			},
		},
		{
			name: "Primitive types masking",
			input: map[string]any{
				"password": 12345,
				"token":    true,
				"secret":   99.9,
			},
			expected: map[string]any{
				"password": mask,
				"token":    mask,
				"secret":   mask,
			},
		},
		{
			name: "Max length for non-sensitive string",
			input: map[string]any{
				"description": "this is a very long description that should be truncated",
			},
			maxLength: 10,
			expected: map[string]any{
				"description": "this is a ...",
			},
		},
		{
			name: "Case insensitive filter params",
			input: map[string]any{
				"Password": "myPassword",
				"TOKEN":    "myToken",
			},
			expected: map[string]any{
				"Password": mask,
				"TOKEN":    mask,
			},
		},
		{
			name: "Array of objects with sensitive fields",
			input: map[string]any{
				"users": []any{
					map[string]any{"name": "user1", "token": "t1"},
					map[string]any{"name": "user2", "token": "t2"},
				},
			},
			expected: map[string]any{
				"users": []any{
					map[string]any{"name": "user1", "token": mask},
					map[string]any{"name": "user2", "token": mask},
				},
			},
		},
		{
			name: "JSON string body with sensitive fields (CombinedRequest format)",
			input: map[string]any{
				"body":  `{"username":"demo@example.com","password":"supersecretpassword"}`,
				"query": nil,
			},
			expected: map[string]any{
				"body":  `{"password":"****","username":"demo@example.com"}`,
				"query": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effMaxLength := maxLength
			if tt.maxLength != 0 {
				effMaxLength = tt.maxLength
			}
			got := FilterSensitiveData(tt.input, filterParams, effMaxLength)
			if !reflect.DeepEqual(got, tt.expected) {
				gotJSON, _ := json.Marshal(got)
				expectedJSON, _ := json.Marshal(tt.expected)
				t.Errorf("FilterSensitiveData() = %s, want %s", string(gotJSON), string(expectedJSON))
			}
		})
	}
}

func TestZerologLogger_AppNameOverride(t *testing.T) {
	var buf bytes.Buffer
	cfg := DefaultConfig()
	cfg.AppName = "my-awesome-app"
	
	l := &ZerologLogger{
		logger:  zerolog.New(&buf).With().Timestamp().Logger(),
		appName: cfg.AppName,
	}

	// Case 1: Payload with different app name should be overridden by primary logger config
	l.Info(context.Background(), "test message", LogPayload{
		AppName: "sub-module-name",
	})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log: %v", err)
	}

	if entry["app_name"] != "my-awesome-app" {
		t.Errorf("expected app_name 'my-awesome-app', got '%v'", entry["app_name"])
	}
}

func TestZerologLogger_AutoExtractTracingFromContext(t *testing.T) {
	var buf bytes.Buffer
	l := &ZerologLogger{
		logger:  zerolog.New(&buf).With().Timestamp().Logger(),
		appName: "test-app",
	}

	ctx := context.Background()
	ctx = tracing.WithTraceID(ctx, "trace-abc-123")
	ctx = tracing.WithSessionID(ctx, "session-xyz-789")

	// Payload without TraceId or SessionId explicitly filled in
	l.Error(ctx, "business error", LogPayload{
		Type:  "ERROR",
		Error: "invalid login credentials",
	})

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log: %v", err)
	}

	if entry["trace_id"] != "trace-abc-123" {
		t.Errorf("expected trace_id 'trace-abc-123', got '%v'", entry["trace_id"])
	}
	if entry["session_id"] != "session-xyz-789" {
		t.Errorf("expected session_id 'session-xyz-789', got '%v'", entry["session_id"])
	}
}

