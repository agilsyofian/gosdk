package main

import (
	"auth-service/app/bootstrap"
	"auth-service/app/server/httpserver"
	userService "auth-service/integration/grpc/user"
	userEvent "auth-service/integration/kafka/user_event"
	"auth-service/module/auth"

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

		// 3. Integrations (outbound clients).
		userService.Integration,
		userEvent.Integration,

		// 4. Business modules — start here.
		auth.Module,
	).Run()
}
