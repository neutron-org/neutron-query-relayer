package config

import (
	"fmt"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/kelseyhightower/envconfig"
	"time"
)

// CosmosQueryRelayerConfig describes configuration of the app
type CosmosQueryRelayerConfig struct {
	NeutronChain NeutronChainConfig
	TargetChain  TargetChainConfig
}

type NeutronChainConfig struct {
	ChainPrefix         string
	RPCAddress          string
	ChainID             string
	GasPrices           string
	HomeDir             string
	SignKeyName         string
	Timeout             time.Duration
	GasAdjustment       float64
	TxBroadcastType     TxBroadcastType
	ConnectionID        string
	ClientID            string
	Debug               bool
	ChainProviderConfig relayer.Chain
}

type TargetChainConfig struct {
	RPCAddress             string
	ChainID                string
	AccountPrefix          string
	ValidatorAccountPrefix string
	HomeDir                string
	Timeout                time.Duration
	ConnectionID           string
	ClientID               string
	Debug                  bool
	ChainProviderConfig    relayer.Chain
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
		return cfg, fmt.Errorf("could not read config from a file: %w", err)
	}

	return cfg, nil
}
