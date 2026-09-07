package bootstrap

import (
	"github.com/agilsyofian/gosdk/example/app/server/grpc"
	"github.com/agilsyofian/gosdk/example/app/server/http"
	"go.uber.org/fx"
)

// Server provides both HTTP and GRPC servers for backward compatibility.
// In new services, prefer using server.HTTP or server.GRPC directly for independent control.
var Server = fx.Options(
	http.HTTP,
	grpc.GRPC,
)
