package config

import (
	"fmt"
	"github.com/vrischmann/envconfig"
)

type CosmosQueryRelayerConfig struct {
	//	TODO: full configuration
	LidoChain struct {
		RPCAddress string `envconfig:"default=tcp://127.0.0.1:26657"`
		//RPCAddress string `envconfig:"default=tcp://public-node.terra.dev:26657"` // for tests only
	}
	TargetChain struct {
		//RPCAddress string `envconfig:"default=tcp://public-node.terra.dev:26657"`
		RPCAddress string `envconfig:"default=tcp://127.0.0.1:26657"`
		ChainID    string `envconfig:"default=columbus-5"`
	}
}

func NewCosmosQueryRelayerConfig() (CosmosQueryRelayerConfig, error) {
	config := CosmosQueryRelayerConfig{}
	if err := envconfig.Init(&config); err != nil {
		return config, fmt.Errorf("failed to init config: %w", err)
	}

	return config, nil
}
