package logger

import (
	"encoding/json"
	"reflect"
	"testing"
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
