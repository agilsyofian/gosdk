package security

import (
	"context"
	"errors"

	"github.com/agilsyofian/gosdk/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Interceptor is the gRPC security adapter, analogous to
// grpc.Middleware. It uses the same SecurityManager instance
// as any other configured transport adapter.
type Interceptor struct {
	manager security.SecurityManager
	config  Config
}

// New constructs a gRPC security Interceptor bound to the given
// SecurityManager and Config.
func New(manager security.SecurityManager, config Config) *Interceptor {
	return &Interceptor{manager: manager, config: config}
}

// authenticate performs the shared extraction/authentication logic
// used by both the unary and stream interceptors, returning a context
// with the resolved SecurityContext attached.
func (i *Interceptor) authenticate(ctx context.Context) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)

	sessionID, credential, err := extractCredential(md, i.config)
	if err != nil {
		if errors.Is(err, security.ErrNoCredential) && !i.config.Required {
			return ctx, nil
		}
		return ctx, mapError(err)
	}

	var securityContext *security.SecurityContext
	if sessionID != "" {
		securityContext, err = i.manager.AuthenticateSession(ctx, sessionID)
	} else {
		securityContext, err = i.manager.Authenticate(ctx, *credential)
	}

	if err != nil {
		if i.config.Required {
			return ctx, mapError(err)
		}
		return ctx, nil
	}

	return security.WithContext(ctx, securityContext), nil
}

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor that
// authenticates the call and injects the resulting SecurityContext
// into the handler's context.
func (i *Interceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		newCtx, err := i.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// wrappedServerStream lets us hand the handler a context carrying the
// resolved SecurityContext, since grpc.ServerStream does not expose a
// settable context directly.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// StreamServerInterceptor returns a grpc.StreamServerInterceptor that
// authenticates the call and injects the resulting SecurityContext
// into the handler's stream context.
func (i *Interceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx, err := i.authenticate(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: newCtx})
	}
}

// RequirePermission returns a check function handlers can call
// (typically as the first line of a handler, or wrapped into a
// per-method interceptor) to enforce a permission via
// SecurityManager.Authorize.
func (i *Interceptor) RequirePermission(ctx context.Context, permission string) error {
	if err := i.manager.Authorize(ctx, permission); err != nil {
		return mapError(err)
	}
	return nil
}

// mapError maps a security error to a gRPC status error. Internal
// authentication details are never included in the message.
func mapError(err error) error {
	switch {
	case errors.Is(err, security.ErrForbidden):
		return status.Error(codes.PermissionDenied, "you do not have permission to perform this action")
	case errors.Is(err, security.ErrUnauthenticated),
		errors.Is(err, security.ErrInvalidCredential),
		errors.Is(err, security.ErrUnsupportedCredential),
		errors.Is(err, security.ErrSessionNotFound),
		errors.Is(err, security.ErrSessionExpired),
		errors.Is(err, security.ErrConflictingCredential),
		errors.Is(err, security.ErrNoCredential):
		return status.Error(codes.Unauthenticated, "authentication required")
	default:
		return status.Error(codes.Unauthenticated, "authentication required")
	}
}
