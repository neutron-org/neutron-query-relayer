package config

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

const loggerPrefix = "LOGGER"

// NewLoggerConfig  initializes a default production config w parameters, overwritten by env vars if necessarry
func NewLoggerConfig() (*zap.Config, error) {
	loggerCfg := zap.NewProductionConfig()
	var cfg zap.Config
	err := envconfig.Process(loggerPrefix, &cfg)
	if err != nil {
		return nil, err
	}
	return &loggerCfg, nil
}
