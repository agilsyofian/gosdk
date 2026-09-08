package bootstrap

import (
	"auth-service/app/security"

	"go.uber.org/fx"
)

// Security builds the app's single security.SecurityManager from the
// authenticator (AuthJWT module) and session store (Session module).
// HTTP and gRPC each need one, so it is provided here once and injected
// into both (server/http.go, server/grpc.go) rather than each transport
// constructing its own — same registry, same behavior, one place to
// change if a second authenticator is added later.
var Security = fx.Module("security",
	fx.Provide(
		security.Manager,
		security.FieldDecryptor,
	),
)
