package user_event

import (
	"github.com/agilsyofian/gosdk/example/modules/auth/port"
	"go.uber.org/fx"
)

var Integration = fx.Module("integration-kafka-user-event",
	fx.Provide(
		fx.Annotate(
			NewUserEventPublisher,
			fx.As(new(port.IUserEventPublisher)),
		),
	),
)
