package port

import (
	"context"

	"auth-service/module/auth/domain"
)

// IUserFinder defines the outbound port for looking up user identities.
// Concrete implementations (such as the gRPC client adapter) live in the integration layer.
type IUserFinder interface {
	GetUser(ctx context.Context, username, deviceID string) (*domain.UserIdentity, error)
}
