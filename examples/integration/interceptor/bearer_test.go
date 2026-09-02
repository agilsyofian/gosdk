package interceptor

import (
	"context"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RSA test key in Base64
const testPrivateKeyB64 = "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDXd266aC+z2jAaDUAPHZ6U0g7PNq4NAV0fwPxyFHzYoxaWz1EO7ZJ8HM/hw4JV2cJ5yhx3CfzSydVdpjN3drRUYpFFmylb7/qDrI3I1neZWDykgSIVolSlY29xWWXNIBCHi0bToQVSeRzqnTQV7MxwlZI92NCB1+uiTu68guK8Z8CJ3Z725lDFn+xZThSEnIl1yzkJ3qAWy2xx6+K9gfMMeTVKbtLrU6Jzd3K4Wg6iXvhMHUnaiqZ3aXOnvIfIlI6guyCZ8zaVlPRV6dZFXOplFwWZLlITMV/bOieMNYqtjVOIuEHXFVvICZZv1mXzEtaJ5luDX07vW7c7pYon1FAJAgMBAAECggEAAVbXgYa2ws01VLQfTiprDctrMUe3dP3VehrXHvd53y1nq1/RuohaWrOb+sBTruGyetqnKPfstvYIKdSrs0/SOb5QtSM+HHpT0fV1cSlaEMXvJn/rgyaNp6gTZjSaIdwzQAL+I/loADJVaXgFkQFhRwakGpf6R2C+dcZ9Ma7hr0UyLqnzOin88c5w4fkcguACukU8IrT4kctHXmN5Km2Naui9NfZ0lYv5WjMeIfVtGroGMllJ7lQSiwDC3PR/BBrIPL+0/ZScrR4UmA5OPwM3fIAMyVhygmq7VQ0sfZskgGyqjEzKjZt3XynrnJIWgS4MviZbvMBzR443Odu2kEQ80QKBgQDtgj5ch9iBZswMZ2az7CO7oqbWfmsF8KcwZJo5sQinZSzuII/ZN41ZJgzZy63XKnRRfGMLpu/jsK6tFDOWRbLusx9IULhBK20iN3HY7+BJ74bt/3s1VaxWN7yIfmzVu4meT1Lwmk6dvBLpAnUSl8ZGccQiGh6wDPryz6Xl0HF4QwKBgQDoPd47AhUC5vY0QVGmIQR1uDnpBBioZRVIn7zMP5gL6reMOoO9N+hLzQb1ySffH/3hyKjO0df4rGo/cC5QGOHlyQYsAoK9/6EZMLg9mlrdMdSLthPRXPz6d3Yy3LaJIZtCew79yTiGqjyr+WzmchAl0wYPr2jlyzBYF5tsVianwwKBgDsgUFwCqYBVUfjIiQXDhIXCkRwWrpf09/oRIV+5Vbkuc8f4LTdbIAIQeeSCUw2ckQZ0G8gbz95z7phBzjHbuha4WtJK3gPDRtDc0qe517EwQvvevRJi+JsiFvo31upCXJkfkDs7ftS/rEl7hh+RzxdcXQJMVFVpddOao+cZRBY3AoGBAOUb67mQ00Oar/ZbHf5jAjBT/yVGOkEd/4I2u51Pe2L+Ofbn3wDzP4JQEHa27FOtDtwIBaiK0aHsDaDpvXadJXV3/FO8Ylin77iXHJsBRHSEbScrB5/6OUOqSHviy1y7P5y1I9PwB4iqNtJ9bRZmHlICPS+KH1TbP6UYHOcikl3bAoGABobo3WhEf22s3tQfYOtn7Q5P50buJDzwM54JTN9TshqXd9CtBaz54TsL0BFFvNPM/lFFlmloB5zS/P7em03lPO2Pj6K5SIqh0qdq5i5DhWoQDFPdkVw1d5IjwZC7r9pwAPRtlwttfFpqybDerNR7xlkhKA89cirHndT60boCFHk="

func setupTestAppConfig() config.AppConfig {
	var cfg config.AppConfig
	cfg.AppName = "test-service"
	cfg.Security.Kid.Signing = "sig_v1"
	cfg.Security.PrivateKey = map[string]string{
		"sig_v1": testPrivateKeyB64,
	}
	cfg.Security.Authenticators.Jwt = jwt.Config{
		Kid:      "sig_v1",
		Issuer:   "auth-server",
		Audience: "services",
		CacheTTL: time.Hour,
	}
	return cfg
}

func TestAuthInterceptor_EnsureTokenOnEveryRequest(t *testing.T) {
	appCfg := setupTestAppConfig()
	tokens := jwt.NewTokenCache()
	interceptor := NewBearerInterceptor(tokens, appCfg, nil)

	unaryInterceptor := interceptor.UnaryClientInterceptor()

	var capturedTokens []string
	dummyInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		authValues := md.Get("authorization")
		require.Len(t, authValues, 1)
		capturedTokens = append(capturedTokens, authValues[0])
		return nil
	}

	// First request: should generate and attach token
	ctx1 := context.Background()
	err := unaryInterceptor(ctx1, "/user.ServiceUser/GetUser", nil, nil, nil, dummyInvoker)
	require.NoError(t, err)
	assert.Len(t, capturedTokens, 1)
	assert.Contains(t, capturedTokens[0], "Bearer ")

	// Second request: should reuse cached token
	ctx2 := context.Background()
	err = unaryInterceptor(ctx2, "/user.ServiceUser/GetUser", nil, nil, nil, dummyInvoker)
	require.NoError(t, err)
	assert.Len(t, capturedTokens, 2)
	assert.Equal(t, capturedTokens[0], capturedTokens[1], "Second request should use cached token")

	// Third request with explicit WithJWT in context: should use explicitly provided token
	explicitToken := "explicit-override-token"
	ctx3 := jwt.WithJWT(context.Background(), explicitToken)
	err = unaryInterceptor(ctx3, "/user.ServiceUser/GetUser", nil, nil, nil, dummyInvoker)
	require.NoError(t, err)
	assert.Len(t, capturedTokens, 3)
	assert.Equal(t, "Bearer "+explicitToken, capturedTokens[2])
}

func TestAuthBearer_Helper(t *testing.T) {
	appCfg := setupTestAppConfig()
	tokens := jwt.NewTokenCache()
	unaryInterceptor := AuthBearer(tokens, appCfg, nil)

	dummyInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.NotEmpty(t, md.Get("authorization"))
		return nil
	}

	err := unaryInterceptor(context.Background(), "/user.ServiceUser/GetUser", nil, nil, nil, dummyInvoker)
	assert.NoError(t, err)
}
