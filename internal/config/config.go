package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// CosmosQueryRelayerConfig describes configuration of the app
type CosmosQueryRelayerConfig struct {
	LidoChain   LidoChainConfig   `yaml:"lido-chain" env-required:"true"`
	TargetChain TargetChainConfig `yaml:"target-chain" env-required:"true"`
}

type LidoChainConfig struct {
	ChainPrefix             string          `yaml:"chain-prefix" env-required:"true"`
	RPCAddress              string          `yaml:"rpc-address" env-required:"true"`
	ChainID                 string          `yaml:"chain-id" env-required:"true"`
	GasPrices               string          `yaml:"gas-prices" env-required:"true"`
	EventSubscriberName     string          `yaml:"event-subscriber-name" env-required:"true"`
	HomeDir                 string          `yaml:"home-dir" env-required:"true"`
	SignKeyName             string          `yaml:"sign-key-name" env-default:"default=default"`
	Timeout                 time.Duration   `yaml:"timeout" env-default:"10s"`
	GasAdjustment           float64         `yaml:"gas-adjustment" env-default:"1.5"`
	TxBroadcastType         TxBroadcastType `yaml:"tx-broadcast-type" env-default:"BroadcastTxAsync"`
	ConnectionID            string          `yaml:"connection-id" env-default:"connection-0"`
	ClientID                string          `yaml:"client-id" env-default:"07-tendermint-0"`
	Debug                   bool            `yaml:"debug" env-default:"false"`
	ChainProviderConfigPath string          `yaml:"chain-provider-config-path" env-default:"./configs/chains/lido.json"`
}

type TargetChainConfig struct {
	RPCAddress              string        `yaml:"rpc-address"`
	ChainID                 string        `yaml:"chain-id"`
	ChainPrefix             string        `yaml:"chain-prefix"`
	HomeDir                 string        `yaml:"home-dir" env-required:"true"`
	Timeout                 time.Duration `yaml:"timeout" env-default:"10s"`
	ConnectionID            string        `yaml:"connection-id" env-default:"connection-0"`
	ClientID                string        `yaml:"client-id" env-default:"07-tendermint-0"`
	Debug                   bool          `yaml:"debug" env-default:"false"`
	ChainProviderConfigPath string        `yaml:"chain-provider-config-path" env-default:"./configs/chains/target.json"`
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
