package service

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/logger"
)

type INotificationService interface {
	PushFCM(ctx context.Context, userID, eventType string) error
}

type NotificationService struct {
	logger logger.Logger
}

func NewService(logger logger.Logger) INotificationService {
	return &NotificationService{
		logger: logger,
	}
}

// PushFCM stands in for a real Firebase Cloud Messaging (or APNs, etc.)
// call. Wiring an actual push provider is outside this SDK's scope —
// this keeps the seam a real implementation would fill (what it
// receives, and where it's called from) without pulling a push
// provider's SDK into the example. Swap the body for a real client
// call; the signature and its caller (handler/messaging.EventUser)
// stay the same.
func (s *NotificationService) PushFCM(ctx context.Context, userID, eventType string) error {
	s.logger.Info(ctx, "notification: push simulated", logger.LogPayload{
		Type: "notification.push",
	}.WithMetadata("userId", userID).WithMetadata("eventType", eventType))

	return nil
}
