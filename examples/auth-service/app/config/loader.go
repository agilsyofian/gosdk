package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

func LoadConfig() (AppConfig, error) {
	v := viper.New()

	v.SetConfigType("yaml")

	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		v.AddConfigPath(configPath)
	}

	v.AddConfigPath(".")
	v.AddConfigPath("config")
	v.AddConfigPath("./config")
	v.AddConfigPath("..")
	v.AddConfigPath("../..")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.BindEnv("app_name", "APP_NAME")

	// 1. Read base config (env.yaml)
	v.SetConfigName("env")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return AppConfig{}, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// 2. If APP_ENV or ENV is set (e.g. "staging", "production"), merge environment overrides
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	if appEnv == "" {
		appEnv = strings.ToLower(os.Getenv("ENV"))
	}

	if appEnv != "" && appEnv != "development" && appEnv != "local" {
		v.SetConfigName("env." + appEnv)
		if err := v.MergeInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return AppConfig{}, fmt.Errorf("failed to merge env.%s config: %w", appEnv, err)
			}
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
