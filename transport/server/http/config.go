package http

import "time"

type Config struct {
	Addr            string         `mapstructure:"addr" validate:"required"`
	ReadTimeout     time.Duration  `mapstructure:"read_timeout" validate:"required"`
	WriteTimeout    time.Duration  `mapstructure:"write_timeout" validate:"required"`
	IdleTimeout     time.Duration  `mapstructure:"idle_timeout" validate:"required"`
	ShutdownTimeout time.Duration  `mapstructure:"shutdown_timeout" validate:"required"`
	Mode            string         `mapstructure:"mode" validate:"required"`
	EnableCors      bool           `mapstructure:"enable_cors"`
	EnableSwagger   bool           `mapstructure:"enable_swagger"`
	ShowErrorDetail bool           `mapstructure:"show_error_detail"`
	Actuator        ActuatorConfig `mapstructure:"actuator"`
}

type ActuatorConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Path       string `mapstructure:"path"`
	HealthPath string `mapstructure:"health_path"`
	InfoPath   string `mapstructure:"info_path"`
}
