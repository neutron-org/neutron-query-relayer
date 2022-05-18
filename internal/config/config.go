package config

import (
	"fmt"
	"github.com/vrischmann/envconfig"
	"time"
)

type CosmosQueryRelayerConfig struct {
	LidoChain struct {
		//LOCAL INTERCHAIN ADAPTER
		ChainPrefix   string        `envconfig:"default=cosmos"`
		RPCAddress    string        `envconfig:"default=tcp://127.0.0.1:26657"`
		ChainID       string        `envconfig:"default=testnet"`
		Timeout       time.Duration `envconfig:"default=5s"`
		GasAdjustment float64       `envconfig:"default=1.5"`
		GasPrices     string        `envconfig:"default=0.5stake"`
		Sender        string        `envconfig:"default=cosmos1l5wq2596y6zrgkza8zpaalqcfaj83c6lgupf82"`

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
			Dir         string `envconfig:"default=/Users/nhpd/.interchain-adapter"`
			//Backend string `envconfig:"default=test"`

			//LOCALTERRA
			//GasPrices string `envconfig:"default=1000uatom"`
		}
	}
	TargetChain struct {
		Timeout     time.Duration `envconfig:"default=5s"`
		RPCAddress  string        `envconfig:"default=tcp://public-node.terra.dev:26657"`
		ChainID     string        `envconfig:"default=columbus-5"`
		ChainPrefix string        `envconfig:"default=terra"`

		//RPCAddress string `envconfig:"default=tcp://rpc.cosmos.network:26657"`
		//RPCAddress string `envconfig:"default=tcp://127.0.0.1:26657"`
		//ChainID     string `envconfig:"default=testnet"`
	}
}

func NewCosmosQueryRelayerConfig() (CosmosQueryRelayerConfig, error) {
	config := CosmosQueryRelayerConfig{}
	if err := envconfig.Init(&config); err != nil {
		return config, fmt.Errorf("failed to init config: %w", err)
	}

	return config, nil
}
