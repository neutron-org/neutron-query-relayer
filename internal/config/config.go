package config

import (
	"fmt"
	"github.com/vrischmann/envconfig"
	"time"
)

// CosmosQueryRelayerConfig describes configuration of the app
type CosmosQueryRelayerConfig struct {
	LidoChain   LidoChainConfig
	TargetChain TargetChainConfig
}

type LidoChainConfig struct {
	//LOCAL INTERCHAIN ADAPTER
	ChainPrefix     string          `envconfig:"default=cosmos"`
	RPCAddress      string          `envconfig:"default=tcp://127.0.0.1:26657"`
	ChainID         string          `envconfig:"default=testnet"`
	Timeout         time.Duration   `envconfig:"default=10s"`
	GasAdjustment   float64         `envconfig:"default=1.5"`
	GasPrices       string          `envconfig:"default=0.5stake"`
	Sender          string          `envconfig:"default=cosmos1dlhd34j0w28gexgajys4jqafp68fxhsezs8urm"`
	TxBroadcastType TxBroadcastType `envconfig:"default=BroadcastTxCommit"`

	//LOCALTERRA
	//ChainPrefix string `envconfig:"default=terra"`
	//RPCAddress  string `envconfig:"default=tcp://127.0.0.1:26657"`
	//ChainID     string `envconfig:"default=localterra"`

	//PUBLIC TERRA
	//ChainPrefix   string        `envconfig:"default=terra"`
	//ChainID    string `envconfig:"default=columbus-5"`
	//RPCAddress string `envconfig:"default=tcp://public-node.terra.dev:26657"` // for tests only

	Keyring struct {
		SignKeyName string `envconfig:"default=test2"`
		Dir         string `envconfig:"default=/Users/nhpd/.gaia-wasm-zoned"`
		//Backend string `envconfig:"default=test"`

		//LOCALTERRA
		//GasPrices string `envconfig:"default=1000uatom"`
	}
}

type TargetChainConfig struct {
	Timeout     time.Duration `envconfig:"default=10s"`
	RPCAddress  string        `envconfig:"default=tcp://public-node.terra.dev:26657"`
	ChainID     string        `envconfig:"default=columbus-5"`
	ChainPrefix string        `envconfig:"default=terra"`

	//RPCAddress string `envconfig:"default=tcp://rpc.cosmos.network:26657"`
	//RPCAddress string `envconfig:"default=tcp://127.0.0.1:26657"`
	//ChainID     string `envconfig:"default=testnet"`
}

type TxBroadcastType string

const (
	BroadcastTxSync   TxBroadcastType = "BroadcastTxSync"
	BroadcastTxAsync  TxBroadcastType = "BroadcastTxAsync"
	BroadcastTxCommit TxBroadcastType = "BroadcastTxCommit"
)

func NewCosmosQueryRelayerConfig() (CosmosQueryRelayerConfig, error) {
	config := CosmosQueryRelayerConfig{}
	if err := envconfig.Init(&config); err != nil {
		return config, fmt.Errorf("failed to init config: %w", err)
	}

	return config, nil
}
