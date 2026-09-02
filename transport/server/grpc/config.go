package grpc

type Config struct {
	Addr              string `mapstructure:"addr"`
	ReflectionEnabled bool   `mapstructure:"reflection_enabled"`
	ShowErrorDetail   bool   `mapstructure:"show_error_detail"`
}
