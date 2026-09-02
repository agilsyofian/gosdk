package server

import (
	"go.uber.org/fx"
)

var HTTP = fx.Module("http",
	fx.Provide(
		fx.Annotate(
			NewHttpHandler,
		),
	),
	fx.Invoke(StartHttp),
)

var GRPC = fx.Module("grpc",
	fx.Provide(
		NewGrpcHandler,
	),
	fx.Invoke(StartGrpc),
)
