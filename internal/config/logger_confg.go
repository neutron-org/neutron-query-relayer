package config

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const loggerPrefix = "LOGGER"

// NewLoggerConfig  initializes a default production config w parameters, overwritten by env vars if present
func NewLoggerConfig() (*zap.Config, error) {
	cfg := zap.NewProductionConfig()
	cfg.Sampling = nil //&zap.SamplingConfig{Initial: 0, Thereafter: 0, Hook: nil}
	cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	//cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	err := envconfig.Process(loggerPrefix, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
