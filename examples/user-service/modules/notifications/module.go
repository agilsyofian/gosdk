package notifications

import (
	"user-service/app/messaging/consumer"
	"user-service/modules/notifications/handler/messaging"
	"user-service/modules/notifications/service"

	"go.uber.org/fx"
)

var Module = fx.Module("module-notifications",
	fx.Provide(
		service.New,
		messaging.New,
		consumer.AsTopicSubscriber(messaging.NewUserEventSubscriber),
		consumer.AsTopicSubscriber(messaging.NewNotificationEventSubscriber),
	),
)
