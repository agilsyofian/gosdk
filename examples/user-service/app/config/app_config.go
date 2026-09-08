package config

import (
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/metrics"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/storage/cache/redis"
	"github.com/agilsyofian/gosdk/storage/database/postgres"
	"github.com/agilsyofian/gosdk/storage/objectstorage/s3"
	clientGrpc "github.com/agilsyofian/gosdk/transport/client/grpc"
	clientHttp "github.com/agilsyofian/gosdk/transport/client/http"
	serverGrpc "github.com/agilsyofian/gosdk/transport/server/grpc"
	serverHttp "github.com/agilsyofian/gosdk/transport/server/http"
)

type AppConfig struct {
	AppName    string `mapstructure:"app_name" validate:"required"`
	AppVersion string `mapstructure:"app_version" validate:"required"`

	Server struct {
		HttpServer serverHttp.Config `mapstructure:"http"`
		GrpcServer serverGrpc.Config `mapstructure:"grpc"`
	} `mapstructure:"server"`

	Security struct {
		Kid struct {
			Signing    string `mapstructure:"signin"`
			Encryption string `mapstructure:"encryption"`
		} `mapstructure:"kid"`
		PrivateKey     map[string]string `mapstructure:"private_key"`
		JWKSURL        string            `mapstructure:"jwks_url"`
		Authenticators struct {
			Jwt jwt.Config `mapstructure:"jwt"`
		} `mapstructure:"authenticators"`
	} `mapstructure:"security"`

	Observability struct {
		Logger  logger.Config  `mapstructure:"logger"`
		Tracer  tracing.Config `mapstructure:"tracer"`
		Metrics metrics.Config `mapstructure:"metrics"`
	} `mapstructure:"observability"`

	Storage struct {
		Cache         redis.Config    `mapstructure:"cache"`
		Postgres      postgres.Config `mapstructure:"postgres"`
		ObjectStorage s3.Config       `mapstructure:"objectstorage"`
	} `mapstructure:"storage"`

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
	Brokers  []string          `mapstructure:"brokers"`
	Topics   map[string]string `mapstructure:"topics"`
	Consumer struct {
		GroupID         string `mapstructure:"group_id"`
		Concurrency     int    `mapstructure:"concurrency"`
		AutoOffsetReset string `mapstructure:"auto_offset_reset"`
	} `mapstructure:"consumer"`
}
