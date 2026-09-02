package main

import (
	"github.com/agilsyofian/gosdk/example/app/bootstrap"
	userService "github.com/agilsyofian/gosdk/example/integration/grpc/user"
	"github.com/agilsyofian/gosdk/example/integration/http/httpbin"
	"github.com/agilsyofian/gosdk/example/messaging/consumer"
	"github.com/agilsyofian/gosdk/example/messaging/publisher"
	"github.com/agilsyofian/gosdk/example/modules/auth"
	"github.com/agilsyofian/gosdk/example/modules/notifications"
	"github.com/agilsyofian/gosdk/example/modules/user"
	"github.com/agilsyofian/gosdk/example/server"
	"go.uber.org/fx"
)

// main assembles the whole example app from fx.Module values. Nothing
// here does real work itself — it only declares which pieces exist and
// lets fx figure out the construction order from their dependencies.
//
// The list below is grouped to match examples/README.md's map of the
// codebase:
//
//  1. Setup/infra   — SDK building blocks (config, logging, DB/cache
//     connections, JWT/session, HTTP+gRPC servers, Kafka client).
//     You wire this once per app and rarely touch it again.
//  2. Integrations  — outbound clients to other services.
//  3. Business modules — the part you actually extend. See
//     examples/modules/README.md for the anatomy of a module and how
//     to add a new one.
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
		bootstrap.Server,
		bootstrap.Messaging,
		server.HTTP,
		server.GRPC,

		// 2. Integrations (outbound clients).
		httpbin.Integration,
		userService.Integration,

		// 3. Messaging
		publisher.Kafka,
		consumer.Kafka,

		// 4. Business modules — start here.
		auth.Module,
		user.Module,
		notifications.Module,
	).Run()
}
