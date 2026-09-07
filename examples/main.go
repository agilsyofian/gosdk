package main

import (
	"github.com/agilsyofian/gosdk/example/app/bootstrap"
	"github.com/agilsyofian/gosdk/example/app/messaging/consumer"
	"github.com/agilsyofian/gosdk/example/app/server/grpcserver"
	"github.com/agilsyofian/gosdk/example/app/server/httpserver"
	userService "github.com/agilsyofian/gosdk/example/integration/grpc/user"
	"github.com/agilsyofian/gosdk/example/integration/http/httpbin"
	userEvent "github.com/agilsyofian/gosdk/example/integration/kafka/user_event"
	"github.com/agilsyofian/gosdk/example/modules/auth"
	"github.com/agilsyofian/gosdk/example/modules/notifications"
	"github.com/agilsyofian/gosdk/example/modules/storage"
	"github.com/agilsyofian/gosdk/example/modules/user"
	"go.uber.org/fx"
)

// main assembles the whole example app from fx.Module values. Nothing
// here does real work itself — it only declares which pieces exist and
// lets fx figure out the construction order from their dependencies.
func main() {
	fx.New(
		// Uncomment to silence fx's own startup/shutdown logging (which
		// constructor ran when, in what order) once you're used to it:
		fx.NopLogger,

		// 1. Setup / infrastructure.
		bootstrap.Config,
		bootstrap.Observability,
		bootstrap.Storage,
		bootstrap.AuthJWT,
		bootstrap.Session,
		bootstrap.Security,
		bootstrap.Messaging,

		// 2. Inbound Transports (HTTP, gRPC servers & Messaging)
		httpserver.HTTP,
		grpcserver.GRPC,
		consumer.Kafka,

		// 3. Integrations (outbound clients).
		httpbin.Integration,
		userService.Integration,
		userEvent.Integration,

		// 4. Business modules — start here.
		auth.Module,
		user.Module,
		storage.Module,
		notifications.Module,
	).Run()
}
