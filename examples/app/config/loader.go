package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

func LoadConfig() (AppConfig, error) {
	v := viper.New()

	v.SetConfigName("env")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("example")
	v.AddConfigPath("..")
	v.AddConfigPath("../example")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.BindEnv("app_name", "APP_NAME")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return AppConfig{}, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config AppConfig
	if err := v.Unmarshal(&config); err != nil {
		return AppConfig{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(config); err != nil {
		return AppConfig{}, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}
