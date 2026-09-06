package interceptor

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/transport/client/grpc"
	grpcPkg "google.golang.org/grpc"
)

// BearerInterceptor resolves the JWT once per request/call and attaches it
// to context or gRPC metadata so downstream integration code can ensure
// valid authentication without knowing about TokenCache at all.
type BearerInterceptor struct {
	tokens *jwt.TokenCache
	cfg    config.AppConfig
	log    logger.Logger
}

func NewBearerInterceptor(tokens *jwt.TokenCache, cfg config.AppConfig, log logger.Logger) *BearerInterceptor {
	return &BearerInterceptor{tokens: tokens, cfg: cfg, log: log}
}

// TokenProvider returns a valid token for outgoing gRPC requests.
func (a *BearerInterceptor) TokenProvider(ctx context.Context) (string, error) {
	return a.tokens.TokenGenerator(
		ctx,
		a.cfg.AppName,
		a.cfg.Security.Authenticators.Jwt,
		a.cfg.Security.Kid.Signing,
		a.cfg.Security.PrivateKey,
		a.log,
	)
}

// UnaryClientInterceptor returns a gRPC unary client interceptor that ensures a valid token before each request.
func (a *BearerInterceptor) UnaryClientInterceptor() grpcPkg.UnaryClientInterceptor {
	return grpc.AuthorizationBearer(a.TokenProvider)
}

// AuthBearer is a helper function to create an authorization unary client interceptor.
func AuthBearer(tokens *jwt.TokenCache, cfg config.AppConfig, log logger.Logger) grpcPkg.UnaryClientInterceptor {
	return NewBearerInterceptor(tokens, cfg, log).UnaryClientInterceptor()
}
