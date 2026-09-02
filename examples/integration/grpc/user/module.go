package user

import (
	"context"

	"go.uber.org/fx"
)

var Integration = fx.Module("integration-user-service",
	fx.Provide(
		fx.Annotate(
			NewClientUser,
			fx.OnStop(func(ctx context.Context, c IClientUser) error {
				if client, ok := c.(*ClientUser); ok {
					return client.Close()
				}
				return nil
			}),
		),
	),
)
