package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agilsyofian/gosdk/messaging/kafka"
	"user-service/modules/notifications/service"
)

type IMessagingHandler interface {
	EventUser(ctx context.Context, msg *kafka.Message) error
	EventNotification(ctx context.Context, msg *kafka.Message) error
}

type Messaging struct {
	notifService service.INotificationService
}

func New(
	notifService service.INotificationService,
) IMessagingHandler {
	return &Messaging{
		notifService: notifService,
	}
}

// EventUser decodes a user_event message and forwards it to the
// notification service. A decode error is returned rather than
// swallowed, so the caller can decide how to treat a malformed message;
// this example does not configure a dead-letter topic, but
// kafka.WithDeadLetterTopic (messaging/kafka/options.go) is where that
// would plug in for a poison message that keeps failing to decode.
func (s *Messaging) EventUser(ctx context.Context, msg *kafka.Message) error {
	var event UserEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("notifications: decode user_event: %w", err)
	}

	return s.notifService.PushFCM(ctx, event.UserID, string(event.EventType))
}

// EventNotification decodes a notification_event message and forwards it
// to the notification service.
func (s *Messaging) EventNotification(ctx context.Context, msg *kafka.Message) error {
	var event NotificationEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("notifications: decode notification_event: %w", err)
	}

	eventType := ""
	if len(event.EventType) > 0 {
		eventType = event.EventType[0]
	}
	return s.notifService.PushFCM(ctx, event.UserID, eventType)
}
