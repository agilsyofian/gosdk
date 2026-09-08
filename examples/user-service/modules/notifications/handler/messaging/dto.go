package messaging

import "encoding/json"

// UserEvent mirrors the JSON shape published by
// messaging/publisher.UserEvent (examples/messaging/publisher/dto.go).
// It is declared again here, independently, on purpose: a consumer
// decodes the wire format (JSON), not the producer's Go type. In a real
// system the producer and consumer are usually separate services and
// must not share a Go package just to agree on a message shape — the
// JSON contract is the interface between them.
type UserEvent struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"`
	UserID    string          `json:"userId"`
	Data      json.RawMessage `json:"data"`
}

// NotificationEvent mirrors the JSON shape published by
// messaging/publisher.NotificationEvent (examples/messaging/publisher/dto.go).
type NotificationEvent struct {
	EventID   string          `json:"eventId"`
	EventType []string        `json:"eventType"`
	UserID    string          `json:"userId"`
	Data      json.RawMessage `json:"data"`
}
