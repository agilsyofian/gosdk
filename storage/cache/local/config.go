package local

import "time"

// Config defines the configuration for local in-memory cache.
type Config struct {
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	Tracing         struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"tracing"`
	Logger struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"logger"`
}

// DefaultConfig returns a Config with default settings.
func DefaultConfig() Config {
	return Config{
		CleanupInterval: 5 * time.Minute,
		Tracing: struct {
			Enabled bool `mapstructure:"enabled"`
		}{
			Enabled: false,
		},
		Logger: struct {
			Enabled bool `mapstructure:"enabled"`
		}{
			Enabled: false,
		},
	}
}
