package s3

import "time"

// Config contains connection and operational settings for S3 / MinIO storage.
type Config struct {
	Endpoint         string        `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`
	AccessKey        string        `mapstructure:"access_key" yaml:"access_key" json:"access_key"`
	SecretKey        string        `mapstructure:"secret_key" yaml:"secret_key" json:"secret_key"`
	Bucket           string        `mapstructure:"bucket" yaml:"bucket" json:"bucket"`
	UseSSL           bool          `mapstructure:"use_ssl" yaml:"use_ssl" json:"use_ssl"`
	Region           string        `mapstructure:"region" yaml:"region" json:"region"`
	AutoCreateBucket bool          `mapstructure:"auto_create_bucket" yaml:"auto_create_bucket" json:"auto_create_bucket"`
	PartSize         uint64        `mapstructure:"part_size" yaml:"part_size" json:"part_size"`
	RequestTimeout   time.Duration `mapstructure:"request_timeout" yaml:"request_timeout" json:"request_timeout"`
	Logger           struct {
		Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	} `mapstructure:"logger" yaml:"logger" json:"logger"`
}

// DefaultConfig returns safe, production-grade defaults for S3 / MinIO.
func DefaultConfig() Config {
	return Config{
		UseSSL:           true,
		AutoCreateBucket: false,
		PartSize:         5 * 1024 * 1024, // 5MB part size for multipart upload
		RequestTimeout:   30 * time.Second,
	}
}

func mergeConfig(def, cfg Config) Config {
	if cfg.Endpoint == "" {
		cfg.Endpoint = def.Endpoint
	}
	if cfg.Region == "" {
		cfg.Region = def.Region
	}
	if cfg.PartSize == 0 {
		cfg.PartSize = def.PartSize
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = def.RequestTimeout
	}
	return cfg
}
