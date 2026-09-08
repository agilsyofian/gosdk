package main

import (
	"auth-service/module/auth"
	"user-service/app/bootstrap"
	"user-service/app/messaging/consumer"
	"user-service/app/server/grpcserver"
	"user-service/app/server/httpserver"
	"user-service/modules/notifications"
	"user-service/modules/storage"
	"user-service/modules/user"

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

		// 4. Business modules — start here.
		auth.Module,
		user.Module,
		storage.Module,
		notifications.Module,
	).Run()
}
