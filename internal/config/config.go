package config

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"time"
)

// CosmosQueryRelayerConfig describes configuration of the app
type CosmosQueryRelayerConfig struct {
	NeutronChain NeutronChainConfig
	TargetChain  TargetChainConfig
}

type NeutronChainConfig struct {
	ChainPrefix     string
	RPCAddr         string
	ChainID         string
	GasPrices       string
	HomeDir         string
	SignKeyName     string
	Timeout         time.Duration
	GasAdjustment   float64
	TxBroadcastType TxBroadcastType
	ConnectionID    string
	ClientID        string
	Debug           bool
	Key             string
	AccountPrefix   string
	KeyringBackend  string
	OutputFormat    string
	SignModeStr     string
}

type TargetChainConfig struct {
	RPCAddr                string
	ChainID                string
	AccountPrefix          string
	ValidatorAccountPrefix string
	HomeDir                string
	Timeout                time.Duration
	ConnectionID           string
	ClientID               string
	Debug                  bool
	Key                    string
	KeyringBackend         string
	OutputFormat           string
	SignModeStr            string
	GasAdjustment          float64
	GasPrices              string
}

type TxBroadcastType string

const (
	BroadcastTxSync   TxBroadcastType = "BroadcastTxSync"
	BroadcastTxAsync  TxBroadcastType = "BroadcastTxAsync"
	BroadcastTxCommit TxBroadcastType = "BroadcastTxCommit"
	EnvPrefix         string          = "" // do we need prefix? it's decreases readableness
)

func NewCosmosQueryRelayerConfig() (CosmosQueryRelayerConfig, error) {
	var cfg CosmosQueryRelayerConfig

	err := envconfig.Process(EnvPrefix, cfg)
	if err != nil {
		return cfg, fmt.Errorf("could not read config from env: %w", err)
	}

	return cfg, nil
}
