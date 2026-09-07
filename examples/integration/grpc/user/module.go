package user

import (
	"context"

	"github.com/agilsyofian/gosdk/example/modules/auth/port"
	"go.uber.org/fx"
)

var Integration = fx.Module("integration-user-service",
	fx.Provide(
		fx.Annotate(
			NewClientUser,
			fx.As(new(port.IUserFinder)),
			fx.OnStop(func(ctx context.Context, finder port.IUserFinder) error {
				if client, ok := finder.(*ClientUser); ok {
					return client.Close()
				}
				return nil
			}),
		),
	),
)
