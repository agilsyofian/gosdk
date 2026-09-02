package config

import (
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_Security(t *testing.T) {
	// Ensure we are in the right directory for the loader to find env.yaml
	// Or we can mock the config file if needed.
	// For now, let's try to run it in the example directory.

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	_ = os.Chdir("../..")

	config, err := LoadConfig()
	assert.NoError(t, err)

	// Check if security is loaded
	assert.NotEmpty(t, config, "should not be empty")
}

func TestLoadConfig_Validation(t *testing.T) {
	// We want to test validation without a valid env.yaml
	v := viper.New()
	// Don't add paths, so ReadInConfig fails or is empty

	var config AppConfig
	err := v.Unmarshal(&config)
	assert.NoError(t, err)

	validate := validator.New()
	err = validate.Struct(config)
	assert.Error(t, err, "Should fail validation if required fields are empty")
	assert.Contains(t, err.Error(), "required")
}
