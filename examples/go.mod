module github.com/agilsyofian/gosdk/example

go 1.26.8

tool github.com/agilsyofian/utils/mapgen

replace (
	github.com/agilsyofian/gosdk/core/errors => ./../core/errors
	github.com/agilsyofian/gosdk/core/metadata => ./../core/metadata
	github.com/agilsyofian/gosdk/core/observability => ./../core/observability
	github.com/agilsyofian/gosdk/resilience => ./../core/resilience
	github.com/agilsyofian/gosdk/security => ./../security
	github.com/agilsyofian/gosdk/security/authenticator/jwt => ./../security/authenticator/jwt
	github.com/agilsyofian/gosdk/security/jwks => ./../security/jwks
	github.com/agilsyofian/gosdk/security/session/redis => ./../security/session/redis
	github.com/agilsyofian/gosdk/storage/cache => ./../storage/cache
	github.com/agilsyofian/gosdk/storage/database => ./../storage/database
	github.com/agilsyofian/gosdk/storage/postgres => ./../storage/database/postgres
	github.com/agilsyofian/gosdk/storage/redis => ./../storage/cache/redis
	github.com/agilsyofian/gosdk/tools/mapgen => ./../tools/mapgen
	github.com/agilsyofian/gosdk/transport/client/grpc => ./../transport/client/grpc
	github.com/agilsyofian/gosdk/transport/client/http => ./../transport/client/http
	github.com/agilsyofian/gosdk/transport/server/grpc => ./../transport/server/grpc
	github.com/agilsyofian/gosdk/transport/server/http => ./../transport/server/http
	github.com/agilsyofian/gosdk/utils => ./../utils
)

require (
	github.com/agilsyofian/gosdk/core/errors v0.0.1
	github.com/agilsyofian/gosdk/core/metadata v0.0.1
	github.com/agilsyofian/gosdk/core/observability v0.0.1
	github.com/agilsyofian/gosdk/resilience v0.0.1 // indirect
	github.com/agilsyofian/gosdk/security v0.0.1
	github.com/agilsyofian/gosdk/security/authenticator/jwt v0.0.1
	github.com/agilsyofian/gosdk/security/jwks v0.0.1
	github.com/agilsyofian/gosdk/security/session/redis v0.0.1
	github.com/agilsyofian/gosdk/storage/cache v0.0.1
	github.com/agilsyofian/gosdk/storage/database v0.0.1
	github.com/agilsyofian/gosdk/storage/postgres v0.0.1
	github.com/agilsyofian/gosdk/storage/redis v0.0.1
	github.com/agilsyofian/gosdk/tools/mapgen v0.0.1
	github.com/agilsyofian/gosdk/transport/client/grpc v0.0.1
	github.com/agilsyofian/gosdk/transport/client/http v0.0.1
	github.com/agilsyofian/gosdk/transport/server/grpc v0.0.1
	github.com/agilsyofian/gosdk/transport/server/http v0.0.1
	github.com/agilsyofian/gosdk/utils v0.0.1
)

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic v1.15.3 // indirect
	github.com/bytedance/sonic/loader v0.5.2 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.15 // indirect
	github.com/gin-contrib/sse v1.1.1 // indirect
	github.com/gin-gonic/gin v1.12.0
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v1.0.1 // indirect
	github.com/go-openapi/jsonreference v1.0.2 // indirect
	github.com/go-openapi/spec v1.0.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.4
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/leodido/go-urn v1.5.0 // indirect
	github.com/lib/pq v1.12.3
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.62.0 // indirect
	github.com/redis/go-redis/v9 v9.22.0 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sony/gobreaker/v2 v2.4.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0
	github.com/stretchr/testify v1.12.1
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/swaggo/files v1.0.1 // indirect
	github.com/swaggo/gin-swagger v1.6.1 // indirect
	github.com/swaggo/swag v1.16.6 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.2 // indirect
	go.mongodb.org/mongo-driver/v2 v2.8.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.69.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/fx v1.24.0
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/arch v0.30.0 // indirect
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260831171406-18b4a7587f8a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260831171406-18b4a7587f8a // indirect
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/segmentio/kafka-go v0.4.47
	google.golang.org/genproto v0.0.0-20260831171406-18b4a7587f8a
)

require (
	github.com/go-openapi/swag/conv v0.29.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.29.1 // indirect
	github.com/go-openapi/swag/loading v0.29.1 // indirect
	github.com/go-openapi/swag/pools v0.29.1 // indirect
	github.com/go-openapi/swag/stringutils v0.29.1 // indirect
	github.com/go-openapi/swag/typeutils v0.29.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.29.1 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	golang.org/x/mod v0.40.0 // indirect
)
