package config

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"time"
)

// CosmosQueryRelayerConfig describes configuration of the app
type CosmosQueryRelayerConfig struct {
	LidoChain   LidoChainConfig   `yaml:"lido-chain" env-required:"true"`
	TargetChain TargetChainConfig `yaml:"target-chain" env-required:"true"`
}

type LidoChainConfig struct {
	ChainPrefix     string          `yaml:"chain-prefix" env-required:"true"`
	RPCAddress      string          `yaml:"rpc-address" env-required:"true"`
	ChainID         string          `yaml:"chain-id" env-required:"true"`
	GasPrices       string          `yaml:"gas-prices" env-required:"true"`
	KeyringDir      string          `yaml:"keyring-dir" env-required:"true"`
	SignKeyName     string          `yaml:"sign-key-name" env-default:"default=default"`
	Timeout         time.Duration   `yaml:"timeout" env-default:"10s"`
	GasAdjustment   float64         `yaml:"gas-adjustment" env-default:"1.5"`
	TxBroadcastType TxBroadcastType `yaml:"tx-broadcast-type" env-default:"BroadcastTxAsync"`
}

type TargetChainConfig struct {
	RPCAddress             string        `yaml:"rpc-address"`
	ChainID                string        `yaml:"chain-id"`
	AccountPrefix          string        `yaml:"account-prefix"`
	ValidatorAccountPrefix string        `yaml:"validator-account-prefix"`
	Timeout                time.Duration `yaml:"timeout" env-default:"10s"`
}

type TxBroadcastType string

const (
	BroadcastTxSync   TxBroadcastType = "BroadcastTxSync"
	BroadcastTxAsync  TxBroadcastType = "BroadcastTxAsync"
	BroadcastTxCommit TxBroadcastType = "BroadcastTxCommit"
)

func NewCosmosQueryRelayerConfig(path string) (CosmosQueryRelayerConfig, error) {
	var cfg CosmosQueryRelayerConfig

	if path == "" {
		return cfg, fmt.Errorf("empty config path (please set CONFIG_PATH env variable)")
	}

	err := cleanenv.ReadConfig(path, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("could not read config from a file: %w", err)
	}

	return cfg, nil
}
