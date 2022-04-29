package config

import (
	"fmt"
	"github.com/vrischmann/envconfig"
)

type CosmosQueryRelayerConfig struct {
	//	TODO
	LidoChain struct {
		RPCAddress string `envconfig:"default='tcp://0.0.0.0:26657'"`
	}
	TargetChain struct {
		RPCAddress string `envconfig:"default='tcp://public-node.terra.dev:26657'"`
		ChainID    string `envconfig:"default='columbus-5'"`
	}
}

func NewCosmosQueryRelayerConfig() (CosmosQueryRelayerConfig, error) {
	config := CosmosQueryRelayerConfig{}
	if err := envconfig.Init(&config); err != nil {
		return config, fmt.Errorf("failed to init config: %w", err)
	}

	return config, nil
}
