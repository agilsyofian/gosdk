package user

import (
	"context"
	"fmt"

	"auth-service/app/config"
	"auth-service/integration/interceptor"
	"auth-service/module/auth/domain"
	pbUser "auth-service/proto/user"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/transport/client/grpc"
)

type ClientUser struct {
	appCfg     config.AppConfig
	client     *grpc.Client
	authClient pbUser.ServiceUserClient
}

func NewClientUser(appCfg config.AppConfig, l logger.Logger) (*ClientUser, error) {

	cfg := appCfg.GrpcIntegrations["user_service"]

	ctx := context.Background()
	client := grpc.New(l, cfg)

	tokens := jwt.NewTokenCache()
	client.Use(interceptor.AuthBearer(tokens, appCfg, l))

	conn, err := client.Dial(ctx)
	if err != nil {
		return nil, fmt.Errorf("dial user client: %w", err)
	}

	authClient := pbUser.NewServiceUserClient(conn)

	return &ClientUser{
		appCfg:     appCfg,
		client:     client,
		authClient: authClient,
	}, nil
}

func (c *ClientUser) Close() error {
	return c.client.Close()
}

func (c *ClientUser) GetUser(ctx context.Context, username, deviceId string) (*domain.UserIdentity, error) {
	resp, err := c.authClient.GetUser(ctx, &pbUser.Request{
		Username: username,
		DeviceId: deviceId,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	return &domain.UserIdentity{
		UserID:            resp.UserId,
		Subject:           resp.Subject,
		Email:             resp.Email,
		PhoneNumber:       resp.PhoneNumber,
		FullName:          resp.FullName,
		DisplayName:       resp.DisplayName,
		AvatarURL:         resp.AvatarUrl,
		Locale:            resp.Locale,
		Timezone:          resp.Timezone,
		Status:            resp.Status,
		DevicesIdentifier: resp.DevicesIdentifier,
		Platform:          resp.Platform,
		DeviceName:        resp.DeviceName,
		PushToken:         resp.PushToken,
		Roles:             resp.Roles,
		Permissions:       resp.Permissions,
		PasswordHash:      resp.PasswordHash,
		BiometricEnabled:  resp.BiometricEnabled,
		BiometricHash:     resp.BiometricHash,
	}, nil
}
