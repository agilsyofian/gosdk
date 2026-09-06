package config

import (
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/security/jwks"
	objectstorage "github.com/agilsyofian/gosdk/storage/objectstorage/s3"
	"github.com/agilsyofian/gosdk/storage/postgres"
	"github.com/agilsyofian/gosdk/storage/redis"
	clientGrpc "github.com/agilsyofian/gosdk/transport/client/grpc"
	clientHttp "github.com/agilsyofian/gosdk/transport/client/http"
	serverGrpc "github.com/agilsyofian/gosdk/transport/server/grpc"
	serverHttp "github.com/agilsyofian/gosdk/transport/server/http"
)

type AppConfig struct {
	AppName    string `mapstructure:"app_name" validate:"required"`
	AppVersion string `mapstructure:"app_version" validate:"required"`

	Server struct {
		HttpServer serverHttp.Config `mapstructure:"http" validate:"required"`
		GrpcServer serverGrpc.Config `mapstructure:"grpc" validate:"required"`
	} `mapstructure:"server" validate:"required"`

	Security struct {
		Kid struct {
			Signing    string `mapstructure:"signin" validate:"required"`
			Encryption string `mapstructure:"encryption" validate:"required"`
		} `mapstructure:"kid" validate:"required"`
		PrivateKey     map[string]string `mapstructure:"private_key" validate:"required"`
		JWKS           jwks.Jwks         `mapstructure:"jwks" validate:"required"`
		Authenticators struct {
			Jwt jwt.Config `mapstructure:"jwt" validate:"required"`
		} `mapstructure:"authenticators" validate:"required"`
	} `mapstructure:"security" validate:"required"`

	Observability struct {
		Logger logger.Config  `mapstructure:"logger" validate:"required"`
		Tracer tracing.Config `mapstructure:"tracer" validate:"required"`
	} `mapstructure:"observability" validate:"required"`

	Storage struct {
		Cache         redis.Config         `mapstructure:"cache" validate:"required"`
		Postgres      postgres.Config      `mapstructure:"postgres" validate:"required"`
		ObjectStorage objectstorage.Config `mapstructure:"objectstorage"`
	} `mapstructure:"storage" validate:"required"`

	HttpIntegrations map[string]clientHttp.Config `mapstructure:"http_integrations"`
	GrpcIntegrations map[string]clientGrpc.Config `mapstructure:"grpc_integrations"`
	Messaging        KafkaIntegration             `mapstructure:"messaging"`

	// add another config here
	ClientInterceptor struct {
		HttpBin struct {
			Username string `mapstructure:"username"`
			Password string `mapstructure:"password"`
		} `mapstructure:"httpbin"`
	} `mapstructure:"client_interceptor"`
}

type KafkaIntegration struct {
	Brokers  []string          `mapstructure:"brokers" validate:"required"`
	Topics   map[string]string `mapstructure:"topics" validate:"required"`
	Consumer struct {
		GroupID         string `mapstructure:"group_id" validate:"required"`
		Concurrency     int    `mapstructure:"concurrency" validate:"required"`
		AutoOffsetReset string `mapstructure:"auto_offset_reset"`
	} `mapstructure:"consumer"`
}
