package local

import "time"

// Config defines the configuration for local in-memory cache.
type Config struct {
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	NumShards       int           `mapstructure:"num_shards"`
	MaxItems        int           `mapstructure:"max_items"`
	MaxMemoryMB     int           `mapstructure:"max_memory_mb"`
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
		NumShards:       32,
		MaxItems:        100_000,
		MaxMemoryMB:     256,
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
