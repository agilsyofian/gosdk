package idempotency

import (
	"encoding/json"
	"time"
)

// Record is the agnostic cached response representation stored for an idempotent operation.
type Record struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body"`
	CreatedAt  time.Time         `json:"created_at"`
}

// Marshal serializes the Record into JSON bytes.
func (r *Record) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// UnmarshalRecord deserializes raw bytes into a Record pointer.
func UnmarshalRecord(data []byte) (*Record, error) {
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// UnmarshalRecordString deserializes a JSON string into a Record pointer.
func UnmarshalRecordString(data string) (*Record, error) {
	return UnmarshalRecord([]byte(data))
}
