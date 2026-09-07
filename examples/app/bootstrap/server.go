package bootstrap

import (
	"github.com/agilsyofian/gosdk/example/app/server/grpcserver"
	"github.com/agilsyofian/gosdk/example/app/server/httpserver"
	"go.uber.org/fx"
)

// Server provides both HTTP and GRPC servers for backward compatibility.
// In new services, prefer using server.HTTP or server.GRPC directly for independent control.
var Server = fx.Options(
	httpserver.HTTP,
	grpcserver.GRPC,
)
