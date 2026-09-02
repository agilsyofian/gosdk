package publisher

import (
	"encoding/json"
)

type UserEventType string

const (
	UserEventCreated UserEventType = "USER:CREATED"
	UserEventUpdated UserEventType = "USER:UPDATED"
	UserEventDeleted UserEventType = "USER:DELETED"
)

type UserEvent struct {
	EventID   string          `json:"eventId"`
	EventType UserEventType   `json:"eventType"`
	UserID    string          `json:"userId"`
	Data      json.RawMessage `json:"data"`
}

func (e UserEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

type NotificationEvent struct {
	EventID   string          `json:"eventId"`
	EventType []string        `json:"eventType"`
	UserID    string          `json:"userId"`
	Data      json.RawMessage `json:"data"`
}

func (e NotificationEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
