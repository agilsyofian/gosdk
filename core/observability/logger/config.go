package logger

import (
	"github.com/rs/zerolog"
)

type Config struct {
	AppName             string        `mapstructure:"app_name"`
	Output              string        `mapstructure:"output" validate:"required"`
	LogDir              string        `mapstructure:"log_dir"` // stdout, stderr, file
	LogLevel            zerolog.Level `mapstructure:"log_level"`
	FilterParams        []string      `mapstructure:"filter_params"`
	MaxLengthParamValue int           `mapstructure:"max_length_param_value"`
}

func DefaultConfig() Config {
	return Config{
		LogLevel:            zerolog.InfoLevel,
		Output:              "stdout",
		MaxLengthParamValue: 256,
	}
}
