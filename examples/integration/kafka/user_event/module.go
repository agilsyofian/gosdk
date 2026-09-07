package user_event

import "go.uber.org/fx"

var Integration = fx.Module("integration-kafka-user-event",
	fx.Provide(
		NewUserEventPublisher,
	),
)
