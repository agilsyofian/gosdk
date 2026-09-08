package notifications

import (
	"go.uber.org/fx"
	"user-service/app/messaging/consumer"
	"user-service/modules/notifications/handler/messaging"
	"user-service/modules/notifications/service"
)

var Module = fx.Module("module-notifications",
	fx.Provide(
		service.New,
		messaging.New,
		consumer.AsTopicSubscriber(messaging.NewUserEventSubscriber),
		consumer.AsTopicSubscriber(messaging.NewNotificationEventSubscriber),
	),
)
