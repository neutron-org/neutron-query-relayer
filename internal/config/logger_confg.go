package config

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

const loggerPrefix = "LOGGER"

// NewLoggerConfig  initializes a default production config w parameters, overwritten by env vars if present
func NewLoggerConfig() (*zap.Config, error) {
	cfg := zap.NewProductionConfig()
	err := envconfig.Process(loggerPrefix, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
