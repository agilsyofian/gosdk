package notifications

import (
	"github.com/agilsyofian/gosdk/example/app/messaging/consumer"
	"github.com/agilsyofian/gosdk/example/modules/notifications/handler/messaging"
	"github.com/agilsyofian/gosdk/example/modules/notifications/service"
	"go.uber.org/fx"
)

var Module = fx.Module("module-notifications",
	fx.Provide(
		service.NewService,
		messaging.NewHandler,
		consumer.AsTopicSubscriber(messaging.NewUserEventSubscriber),
		consumer.AsTopicSubscriber(messaging.NewNotificationEventSubscriber),
	),
)
