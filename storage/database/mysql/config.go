package mysql

import "time"

// Config holds connection parameters that are uniform across all drivers.
// Fields not relevant to a particular driver can simply be left empty.
type Config struct {
	Host     string `mapstructure:"host" validate:"required"`
	Port     int    `mapstructure:"port" validate:"required"`
	User     string `mapstructure:"user" validate:"required"`
	Password string `mapstructure:"password" validate:"required"`
	DBName   string `mapstructure:"db_name" validate:"required"` // for MySQL: database name

	// SSL/TLS
	SSLMode string `mapstructure:"ssl_mode" validate:"required"` // e.g. "disable", "require"

	// Connection pool
	MaxOpenConns    int           `mapstructure:"max_open_conns" validate:"required,gte=0"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns" validate:"required,gte=0"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" validate:"required,gte=0"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time" validate:"required,gte=0"`

	// Custom DSN (if set, it's used directly and other fields are ignored)
	DSN string `mapstructure:"dsn"`

	Tracing struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"tracing"`
	Logger struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"logger"`
}
