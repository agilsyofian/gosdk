package http

import "time"

type Config struct {
	BasePath                   string                `mapstructure:"base_path"`
	Addr                       string                `mapstructure:"addr" validate:"required"`
	ReadTimeout                time.Duration         `mapstructure:"read_timeout" validate:"required"`
	WriteTimeout               time.Duration         `mapstructure:"write_timeout" validate:"required"`
	IdleTimeout                time.Duration         `mapstructure:"idle_timeout" validate:"required"`
	ShutdownTimeout            time.Duration         `mapstructure:"shutdown_timeout" validate:"required"`
	ShutdownDelay              time.Duration         `mapstructure:"shutdown_delay"`
	Mode                       string                `mapstructure:"mode" validate:"required"`
	EnableCors                 bool                  `mapstructure:"enable_cors"`
	Cors                       CorsConfig            `mapstructure:"cors"`
	DisableBlockMethodOverride bool                  `mapstructure:"disable_block_method_override"`
	DisableBlockSpecialChars   bool                  `mapstructure:"disable_block_special_chars"`
	AllowedSpecialCharsRegex   string                `mapstructure:"allowed_special_chars_regex"`
	SecurityHeaders            SecurityHeadersConfig `mapstructure:"security_headers"`
	EnableSwagger              bool                  `mapstructure:"enable_swagger"`
	ShowErrorDetail            bool                  `mapstructure:"show_error_detail"`
	LoggerExcludeRoutes        []string              `mapstructure:"logger_exclude_routes"`
	Actuator                   ActuatorConfig        `mapstructure:"actuator"`
}

type SecurityHeadersConfig struct {
	Disabled              bool          `mapstructure:"disabled"`
	HSTSMaxAge            time.Duration `mapstructure:"hsts_max_age"`
	DisableHSTSSubdomains bool          `mapstructure:"disable_hsts_subdomains"`
	DisableHSTSPreload    bool          `mapstructure:"disable_hsts_preload"`
	ContentSecurityPolicy string        `mapstructure:"content_security_policy"`
	FrameOptions          string        `mapstructure:"frame_options"`
	ReferrerPolicy        string        `mapstructure:"referrer_policy"`
	DisableNoCache        bool          `mapstructure:"disable_no_cache"`
	SanitizeResponse      bool          `mapstructure:"sanitize_response"`
}

type CorsConfig struct {
	AllowOrigins     []string      `mapstructure:"allow_origins"`
	AllowMethods     []string      `mapstructure:"allow_methods"`
	AllowHeaders     []string      `mapstructure:"allow_headers"`
	ExposeHeaders    []string      `mapstructure:"expose_headers"`
	AllowCredentials bool          `mapstructure:"allow_credentials"`
	MaxAge           time.Duration `mapstructure:"max_age"`
}

type ActuatorConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Path          string `mapstructure:"path"`
	HealthPath    string `mapstructure:"health_path"`
	LivenessPath  string `mapstructure:"liveness_path"`
	ReadinessPath string `mapstructure:"readiness_path"`
	InfoPath      string `mapstructure:"info_path"`
}
