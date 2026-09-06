package grpc

import (
	"time"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	grpcSecurity "github.com/agilsyofian/gosdk/transport/server/grpc/security"
)

type Config struct {
	Addr                  string               `mapstructure:"addr"`
	ReflectionEnabled     bool                 `mapstructure:"reflection_enabled"`
	ShowErrorDetail       bool                 `mapstructure:"show_error_detail"`
	ShutdownTimeout       time.Duration        `mapstructure:"shutdown_timeout"`
	MaxConcurrentStreams  uint32               `mapstructure:"max_concurrent_streams"`
	MaxReceiveMessageSize int                  `mapstructure:"max_receive_message_size"`
	MaxSendMessageSize    int                  `mapstructure:"max_send_message_size"`
	ConnectionTimeout     time.Duration        `mapstructure:"connection_timeout"`
	Keepalive             *KeepaliveConfig     `mapstructure:"keepalive"`
	HealthCheckEnabled    bool                 `mapstructure:"health_check_enabled"`
	Metadata              coremetadata.Config  `mapstructure:"metadata"`
	Security              *grpcSecurity.Config `mapstructure:"security"`
}

// KeepaliveConfig defines keepalive parameters and enforcement policy for gRPC server.
type KeepaliveConfig struct {
	MaxConnectionIdle     time.Duration `mapstructure:"max_connection_idle"`
	MaxConnectionAge      time.Duration `mapstructure:"max_connection_age"`
	MaxConnectionAgeGrace time.Duration `mapstructure:"max_connection_age_grace"`
	Time                  time.Duration `mapstructure:"time"`
	Timeout               time.Duration `mapstructure:"timeout"`
	MinTime               time.Duration `mapstructure:"min_time"`
	PermitWithoutStream   bool          `mapstructure:"permit_without_stream"`
}

// DefaultConfig returns production-ready default configurations for gRPC server.
func DefaultConfig() Config {
	return Config{
		Addr:                  ":50051",
		ShutdownTimeout:       30 * time.Second,
		MaxConcurrentStreams:  1000,
		MaxReceiveMessageSize: 16 * 1024 * 1024, // 16MB
		MaxSendMessageSize:    16 * 1024 * 1024, // 16MB
		ConnectionTimeout:     120 * time.Second,
		HealthCheckEnabled:    true,
	}
}

// MetadataConfig defines declarative header mapping and capture rules for gRPC.
type MetadataConfig = coremetadata.Config
