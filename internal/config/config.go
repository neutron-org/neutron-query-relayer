package config

import (
	"fmt"
	"github.com/vrischmann/envconfig"
	"time"
)

type CosmosQueryRelayerConfig struct {
	//	TODO: full configuration
	LidoChain struct {
		//LOCAL INTERCHAIN ADAPTER
		ChainPrefix string `envconfig:"default=cosmos"`
		RPCAddress  string `envconfig:"default=tcp://127.0.0.1:26657"`
		ChainID     string `envconfig:"default=testnet"`

		//LOCALTERRA
		//ChainPrefix string `envconfig:"default=terra"`
		//RPCAddress  string `envconfig:"default=tcp://127.0.0.1:26657"`
		//ChainID     string `envconfig:"default=localterra"`

		//PUBLIC TERRA
		//ChainPrefix   string        `envconfig:"default=terra"`
		//ChainID    string `envconfig:"default=columbus-5"`
		//RPCAddress string `envconfig:"default=tcp://public-node.terra.dev:26657"` // for tests only

		Timeout       time.Duration `envconfig:"default=5s"`
		GasAdjustment float64       `envconfig:"default=1.5"`
		Keyring       struct {
			Dir     string `envconfig:"default=keys"`
			Backend string `envconfig:"default=test"`

			//LOCALTERRA
			//GasPrices string `envconfig:"default=1000uatom"`

			//LOCAL INTERCHAIN ADAPTER
			GasPrices string `envconfig:"default=0.5stake"`
		}

		Sender string `envconfig:"default=TODO"`
	}
	TargetChain struct {
		Timeout time.Duration `envconfig:"default=5s"`
		//RPCAddress string `envconfig:"default=tcp://rpc.cosmos.network:26657"`
		RPCAddress string `envconfig:"default=tcp://public-node.terra.dev:26657"`
		//RPCAddress string `envconfig:"default=http://167.99.25.150:26657"`
		//RPCAddress string `envconfig:"default=tcp://127.0.0.1:26657"`
		ChainID string `envconfig:"default=columbus-5"`
		//ChainID     string `envconfig:"default=testnet"`
		ChainPrefix string `envconfig:"default=terra"`
	}
}

func NewCosmosQueryRelayerConfig() (CosmosQueryRelayerConfig, error) {
	config := CosmosQueryRelayerConfig{}
	if err := envconfig.Init(&config); err != nil {
		return config, fmt.Errorf("failed to init config: %w", err)
	}

	return config, nil
}
