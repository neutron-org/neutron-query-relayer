package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"

	"github.com/neutron-org/neutron-query-relayer/internal/registry"
)

// NeutronQueryRelayerConfig describes configuration of the app
type NeutronQueryRelayerConfig struct {
	NeutronChain                *NeutronChainConfig      `split_words:"true"`
	TargetChain                 *TargetChainConfig       `split_words:"true"`
	Registry                    *registry.RegistryConfig `split_words:"true"`
	AllowTxQueries              bool                     `required:"true" split_words:"true"`
	AllowKVCallbacks            bool                     `required:"true" split_words:"true"`
	MinKvUpdatePeriod           uint64                   `split_words:"true" default:"0"`
	StoragePath                 string                   `split_words:"true"`
	CheckSubmittedTxStatusDelay uint64                   `split_words:"true" default:"10"`
	QueriesTaskQueueCapacity    int                      `split_words:"true" default:"10000"`
	PrometheusPort              uint16                   `split_words:"true" default:"9999"`
}

const EnvPrefix string = "RELAYER"

type NeutronChainConfig struct {
	RPCAddr        string        `required:"true" split_words:"true"`
	RESTAddr       string        `required:"true" split_words:"true"`
	ChainID        string        `required:"true" split_words:"true"`
	HomeDir        string        `required:"true" split_words:"true"`
	SignKeyName    string        `required:"true" split_words:"true"`
	Timeout        time.Duration `required:"true" split_words:"true"`
	GasPrices      string        `required:"true" split_words:"true"`
	GasLimit       uint64        `split_words:"true" default:"0"`
	GasAdjustment  float64       `required:"true" split_words:"true"`
	ConnectionID   string        `required:"true" split_words:"true"`
	ClientID       string        `required:"true" split_words:"true"`
	Debug          bool          `required:"true" split_words:"true"`
	KeyringBackend string        `required:"true" split_words:"true"`
	OutputFormat   string        `required:"true" split_words:"true"`
	SignModeStr    string        `required:"true" split_words:"true"`
}

type TargetChainConfig struct {
	RPCAddr                string        `required:"true" split_words:"true"`
	ChainID                string        `required:"true" split_words:"true"`
	AccountPrefix          string        `required:"true" split_words:"true"`
	ValidatorAccountPrefix string        `required:"true" split_words:"true"`
	Timeout                time.Duration `required:"true" split_words:"true"`
	ConnectionID           string        `required:"true" split_words:"true"`
	ClientID               string        `required:"true" split_words:"true"`
	Debug                  bool          `required:"true" split_words:"true"`
	OutputFormat           string        `required:"true" split_words:"true"`
}

func NewNeutronQueryRelayerConfig() (NeutronQueryRelayerConfig, error) {
	var cfg NeutronQueryRelayerConfig

	err := envconfig.Process(EnvPrefix, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("could not read config from env: %w", err)
	}
	return cfg, nil
}
