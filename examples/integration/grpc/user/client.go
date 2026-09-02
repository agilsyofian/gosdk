package user

import (
	"context"
	"fmt"

	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/integration/interceptor"
	"github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/transport/client/grpc"
)

type IClientUser interface {
	GetUser(ctx context.Context, username, deviceId string) (*user.Response, error)
}

type ClientUser struct {
	appCfg     config.AppConfig
	client     *grpc.Client
	authClient user.ServiceUserClient
}

func NewClientUser(appCfg config.AppConfig, l logger.Logger) (IClientUser, error) {

	cfg := appCfg.GrpcIntegrations["user"]

	ctx := context.Background()
	client := grpc.New(l, cfg)

	tokens := jwt.NewTokenCache()
	client.Use(interceptor.AuthBearer(tokens, appCfg, l))

	conn, err := client.Dial(ctx)
	if err != nil {
		return nil, fmt.Errorf("dial user client: %w", err)
	}

	authClient := user.NewServiceUserClient(conn)

	return &ClientUser{
		appCfg:     appCfg,
		client:     client,
		authClient: authClient,
	}, nil
}

func (c *ClientUser) Close() error {
	return c.client.Close()
}

func (c *ClientUser) GetUser(ctx context.Context, username, deviceId string) (*user.Response, error) {
	return c.authClient.GetUser(ctx, &user.Request{
		Username: username,
		DeviceId: deviceId,
	})
}
