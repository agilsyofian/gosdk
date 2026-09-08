package port

import (
	"context"
)

type UserEventType string

const (
	UserEventCreated UserEventType = "user.created"
	UserEventUpdated UserEventType = "user.updated"
	UserEventDeleted UserEventType = "user.deleted"
)

type UserEvent struct {
	EventID   string        `json:"event_id"`
	EventType UserEventType `json:"event_type"`
	UserID    string        `json:"user_id"`
	Data      []byte        `json:"data"`
}

type IUserEventPublisher interface {
	PublishUserEvent(ctx context.Context, event UserEvent) error
}
