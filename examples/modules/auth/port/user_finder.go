package port

import (
	"context"

	"github.com/agilsyofian/gosdk/example/modules/auth/domain"
)

// IUserFinder defines the outbound port for looking up user identities.
// Concrete implementations (such as the gRPC client adapter) live in the integration layer.
type IUserFinder interface {
	GetUser(ctx context.Context, username, deviceID string) (*domain.UserIdentity, error)
}
