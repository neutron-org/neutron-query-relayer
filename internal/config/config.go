package config

import (
	"fmt"
	"github.com/vrischmann/envconfig"
)

type CosmosQueryRelayerConfig struct {
	//	TODO
}

func NewCosmosQueryRelayerConfig() (CosmosQueryRelayerConfig, error) {
	config := CosmosQueryRelayerConfig{}
	if err := envconfig.Init(&config); err != nil {
		return config, fmt.Errorf("failed to init config: %w", err)
	}

	return config, nil
}
