package grpc

import (
	"time"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
)

type Config struct {
	Addr              string              `mapstructure:"addr"`
	ReflectionEnabled bool                `mapstructure:"reflection_enabled"`
	ShowErrorDetail   bool                `mapstructure:"show_error_detail"`
	ShutdownTimeout   time.Duration       `mapstructure:"shutdown_timeout"`
	Metadata          coremetadata.Config `mapstructure:"metadata"`
}

// MetadataConfig defines declarative header mapping and capture rules for gRPC.
type MetadataConfig = coremetadata.Config
