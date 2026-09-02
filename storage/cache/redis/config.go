package redis

// Config defines the configuration for Redis connection.
type Config struct {
	Type     string   `mapstructure:"type" validate:"required,oneof=standalone cluster"`
	Addrs    []string `mapstructure:"addrs" validate:"required,min=1"`
	Username string   `mapstructure:"username"`
	Password string   `mapstructure:"password"`
	DB       int      `mapstructure:"db"`
	Tracing  struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"tracing"`
	Logger struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"logger"`
}

func DefaultConfig() Config {
	return Config{
		Type:     "standalone",
		Addrs:    []string{"localhost:6379"},
		Username: "",
		Password: "",
		DB:       0,
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
