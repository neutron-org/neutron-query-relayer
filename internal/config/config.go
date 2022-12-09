package config

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/kelseyhightower/envconfig"

	"github.com/neutron-org/neutron-query-relayer/internal/registry"
)

// NeutronQueryRelayerConfig describes configuration of the app
type NeutronQueryRelayerConfig struct {
	NeutronChain                *NeutronChainConfig      `split_words:"true"`
	TargetChain                 *TargetChainConfig       `split_words:"true"`
	Keyring                     *KeyringConfig           `split_words:"true"`
	Registry                    *registry.RegistryConfig `split_words:"true"`
	AllowTxQueries              bool                     `required:"true" split_words:"true"`
	AllowKVCallbacks            bool                     `required:"true" split_words:"true"`
	MinKvUpdatePeriod           uint64                   `split_words:"true" default:"0"`
	StoragePath                 string                   `required:"true" split_words:"true"`
	CheckSubmittedTxStatusDelay time.Duration            `split_words:"true" default:"10s"`
	QueriesTaskQueueCapacity    int                      `split_words:"true" default:"10000"`
	PrometheusPort              uint16                   `split_words:"true" default:"9999"`
	InitialTxSearchOffset       uint64                   `split_words:"true" default:"0"`
}

const EnvPrefix string = "RELAYER"

// NeutronChainConfig TODO: research if HomeDir parameter is needed for something but keys (so it might be optional and ignored for memory keyring)
type NeutronChainConfig struct {
	RPCAddr       string        `required:"true" split_words:"true"`
	RESTAddr      string        `required:"true" split_words:"true"`
	HomeDir       string        `required:"true" split_words:"true"`
	Timeout       time.Duration `split_words:"true" default:"10s"`
	GasPrices     string        `required:"true" split_words:"true"`
	GasLimit      uint64        `split_words:"true" default:"0"`
	GasAdjustment float64       `required:"true" split_words:"true"`
	ConnectionID  string        `required:"true" split_words:"true"`
	Debug         bool          `split_words:"true" default:"false"`
	OutputFormat  string        `split_words:"true" default:"json"`
	SignModeStr   string        `split_words:"true" default:"direct"`
}

type KeyringConfig struct {
	KeyName   string `split_words:"true"`
	KeySeed   string `split_words:"true"`
	KeyHdPath string `split_words:"true"`
	Backend   string `required:"true" split_words:"true"`
	Password  string `split_words:"true"`
}

type TargetChainConfig struct {
	RPCAddr                string        `required:"true" split_words:"true"`
	AccountPrefix          string        `required:"true" split_words:"true"`
	ValidatorAccountPrefix string        `required:"true" split_words:"true"`
	Timeout                time.Duration `split_words:"true" default:"10s"`
	Debug                  bool          `split_words:"true" default:"false"`
	OutputFormat           string        `split_words:"true" default:"json"`
}

func NewNeutronQueryRelayerConfig(logger *zap.Logger) (NeutronQueryRelayerConfig, error) {
	var cfg NeutronQueryRelayerConfig

	err := envconfig.Process(EnvPrefix, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("could not read config from env: %w", err)
	}

	proxy, err := setupProxy(cfg.NeutronChain.RPCAddr, logger)
	if err != nil {
		return cfg, fmt.Errorf("could not start tendermint-workaround reverse proxy for Neutron RPC %s: %w", cfg.NeutronChain.RPCAddr, err)
	}
	cfg.NeutronChain.RPCAddr = proxy

	proxy, err = setupProxy(cfg.TargetChain.RPCAddr, logger)
	if err != nil {
		return cfg, fmt.Errorf("could not start tendermint-workaround reverse proxy for Target RPC %s: %w", cfg.TargetChain.RPCAddr, err)
	}
	cfg.TargetChain.RPCAddr = proxy

	return cfg, nil
}

/*
 * this function spawns a workaround proxy which fights bug fixed in tendermint on this commit:
 * https://github.com/tendermint/tendermint/commit/8e90d294ca7cb2460bd31cc544639a6ea7efbe9d
 * ⠀⠀⠀⠀⣾⠱⣼⣿⣿⣿⣿⣷⣿⣿⣿⡿⢓⣽⣿⣿⣿⣿⣿⣿⣿⣿⣿⢿⣌⢄⠑⠀⠀⢀⠀⠀⠀⣀⠈⠀⠀⠀⠀⠐⠗⡷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡷⣧⠌⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
 * ⠀⠀⣠⣮⣿⢎⣿⣿⣿⣿⣿⣿⣿⠷⡁⠆⢱⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣮⣎⣈⣈⣌⣿⣌⣌⣩⢌⠀⠀⠀⠀⠀⠀⠐⠱⣷⣿⣿⣿⣿⣿⣿⠟⣿⢀⠐⠏⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
 * ⠀⣠⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠓⠀⢀⣬⣿⣿⣿⣿⣿⣿⡿⠳⠇⢙⣙⣽⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣾⣎⣯⠈⠀⠀⠀⠀⠱⣷⣿⣿⣿⣿⢏⣿⣀⠀⠇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
 * ⠀⣸⣿⣿⣿⣿⣿⣿⣿⣿⠿⠁⠀⡬⣿⣿⣿⣿⣷⣿⣿⣿⠋⠀⢚⣝⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣎⠈⢈⠈⠀⠀⠱⣿⣿⣿⣿⣿⣰⠀⣀⠩⠄⠀⠀⠀⠀⠀⠀⠀⠀
 * ⣀⣿⣿⣿⣿⣿⣷⣿⣿⠿⠀⢀⠶⣬⣿⣿⡷⠓⣾⣿⣿⣏⣬⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⢪⠀⠀⠐⣷⣿⣿⣿⠞⠈⣼⣿⠌⠀⠀⠀⠀⠀⠀⠀⠀
 * ⣸⣿⣿⣿⡿⢁⣿⣿⠓⠀⣀⢀⣼⣩⣿⣿⢌⣼⣻⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠄⠀⠀⠀⠐⣿⣿⣿⣿⠌⠷⣿⠏⠀⠀⠀⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣯⣿⣿⢏⠀⠀⠀⠳⢓⣿⣿⠟⣹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣮⣌⠈⠀⡰⣿⣿⣿⣯⠃⡰⠇⠀⠀⠀⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⣿⣿⡿⠳⢀⠜⣳⣿⣿⡳⣬⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠳⣿⣿⣿⣿⣿⣿⣿⢿⠃⠀⠀⣳⣿⣿⡿⣬⠀⣷⣮⢯⠈⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⣿⣿⠃⢐⣿⠷⣿⡿⣉⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠿⠓⠓⠓⠀⠀⠰⠑⣿⠓⣿⣿⠿⣳⠁⠀⠀⣰⣿⣿⣃⣿⠀⡰⣷⣿⣱⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⡿⠁⠀⠀⠑⣿⢗⣸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⡳⠀⠀⠀⠀⠈⣀⢌⠀⠀⠀⠁⡰⣷⣿⠃⠀⢀⠀⠀⣰⣿⣿⣿⣿⠀⠀⣼⣿⡿⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⠃⡀⠀⢀⢎⠀⡸⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣌⣌⣎⢌⢈⠈⠀⠀⢘⠀⠀⠀⠀⠀⠀⠀⠀⠀⣼⠀⠀⣾⣿⣿⣿⣿⢌⣾⣿⣿⠏⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⡿⢀⠌⣠⡿⠁⠐⣡⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣝⣙⠝⣽⢟⢝⢀⠁⠑⡣⡦⠓⠀⠀⠀⠀⠀⠀⠀⢀⣼⠏⠀⣨⣿⣿⣿⣿⣿⣿⣿⣿⣿⣃⠀⠀⠀⠀⠀
 * ⣷⣿⣿⣿⠏⣼⠀⣸⢇⠀⣈⣷⡿⣳⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡹⣓⣟⠐⠀⠀⠀⢀⣮⣿⣿⣿⣿⣾⣬⢟⠈⠁⠀⣿⣿⣿⣿⣿⣿⠻⣿⣿⣿⢟⠌⠀⡠⣏⠀
 * ⣾⣿⣿⣿⠏⠏⠌⣿⠞⣨⠗⣼⠉⠓⣼⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣾⣿⣎⠈⠀⣈⣿⣿⣿⣿⣿⣷⣿⣿⣿⣿⠀⠀⣿⣿⣿⠟⠟⣕⠁⠐⡳⣿⣿⠋⣈⣮⠌⠀
 * ⣿⣿⣿⣿⣿⠏⣯⣿⣫⣿⠾⠀⠀⠘⡻⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠷⢳⠳⡳⣿⣿⣿⣿⣯⣿⣿⣿⣿⠷⠇⠀⡰⠰⣿⣿⣿⠆⠀⣿⣿⣿⣿⣿⣾⠎⠀⠀⣱⣿⣿⣿⣿⠏⠀
 * ⣰⣿⣿⣿⣷⠇⡿⣷⣿⣿⣧⡧⡦⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⣿⣿⣿⣿⣿⠣⠁⠀⠀⠀⠀⣷⣿⣿⣿⣿⣿⣿⡿⠇⠀⠀⠀⠀⠀⣰⣿⣿⠎⠀⣿⣿⣿⣿⣿⣿⣯⠀⠀⠀⣿⣿⣿⣿⠂⠀
 * ⣰⣿⣿⣿⠄⣨⣯⣾⣿⣿⠀⠀⢀⡾⣹⣿⣿⣿⣿⣿⣿⣿⢓⣶⣿⣿⣿⣿⠝⠀⠀⠀⠀⠀⢀⣠⣿⣿⣿⣷⣿⣿⠏⠀⠀⠀⠀⠀⠀⠀⣿⣿⣿⠈⡱⣿⣿⣿⣿⣿⠟⣎⠀⠀⣿⣿⣿⣿⠏⠀
 * ⠾⣿⣿⣿⠎⣿⣿⣿⣿⢟⡜⠆⠸⠱⣿⣿⣿⣿⣿⣿⡿⠁⠀⣿⣿⣿⣿⣿⠀⠀⠀⠀⠀⠀⠀⣸⣿⠷⠁⠒⠑⣿⠏⠀⠀⠀⠀⠀⠀⣰⣿⣿⣿⠁⠀⣳⣿⣿⣿⣿⣏⣷⠎⠀⣿⣿⣿⣿⠏⠀
 * ⣿⣿⣿⣿⠌⣱⣿⣿⣿⣿⠏⢀⣦⣾⣿⣿⣿⣿⣿⠿⣀⠀⢎⣼⣿⣿⣿⣿⠀⠀⠀⠀⠀⡈⠀⣿⣿⠗⠀⠀⠀⣰⣿⠈⠀⠀⠀⠀⠠⣿⣿⣿⠗⠀⠀⠰⣿⣿⣿⣿⣿⣾⠏⣰⣿⣿⠉⠐⠀⡏
 * ⣿⣿⣿⣿⠉⣰⣿⣿⣿⣿⣿⣾⣿⣿⣼⣿⣿⣿⣿⢃⠀⠀⣿⣻⣿⣿⣿⣿⣈⠈⠀⠀⠀⠀⣼⣿⣿⣿⢎⠀⣬⠐⠀⠁⣀⢌⠀⣀⣿⣿⣿⠷⠀⠀⠀⠀⣳⣿⣿⣿⣷⡿⣫⣿⣿⣿⠏⠀⠀⡀
 * ⣿⣿⣿⣿⠏⣰⣿⣿⣿⣿⣿⣿⣷⣿⣿⣿⣿⣿⠏⠀⠀⡀⣫⣿⣿⣿⣿⣿⣿⣿⣿⣼⣯⣿⣿⣿⣿⣿⣿⣾⠆⠀⠀⠀⠐⣿⣿⣿⣿⣿⢷⠀⠀⠀⠀⠀⣸⣿⣿⠟⠐⣿⣿⣿⣿⣿⠏⠀⠀⠀
 * ⣿⣿⣿⣿⣿⠐⢏⠏⣿⣿⣿⣿⠰⡿⣿⣷⣿⣿⠁⠀⢀⠀⣳⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣳⣿⣿⠿⠀⠀⠀⠀⠀⣿⣿⣿⣿⡟⠀⠀⠀⠀⠀⣀⣿⣿⣿⠀⠀⣰⣿⡿⣷⣿⠃⠀⠀⠀
 * ⣿⣿⣿⣿⣿⣠⣾⠁⠏⠗⣿⠏⠀⠀⢏⣰⠟⠐⠏⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠇⣿⣿⣿⠏⠀⠀⠀⠀⣠⣿⣿⡿⠑⠀⠀⠀⠀⠀⢀⣾⣿⣿⠿⠀⠀⣰⣿⣿⣆⠐⢷⠀⠀⠀
 * ⣳⣿⣿⣿⣿⠰⣿⢌⠑⠆⠟⣳⠀⠀⠑⠐⠁⠀⢀⠀⠀⠀⠱⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠓⣰⣿⣿⣿⣏⣌⢌⠀⣠⣿⣿⠿⠁⠀⠀⠀⠀⠀⣈⣾⣿⣿⣿⢟⠈⠀⣼⣿⣿⠀⠀⠀⠀⠀⠀
 * ⣾⣿⣿⣿⣿⠏⣿⣿⠏⡦⠀⠱⠀⠀⣰⠈⡀⠀⣸⠌⠀⡀⠀⣿⣳⣿⣿⣿⣿⣿⣿⠿⡻⠁⣨⣿⣿⣿⣿⣿⣿⣿⡌⠌⣰⣿⠃⠀⠀⠀⠀⣌⣾⣿⣿⣿⣿⣿⣷⠀⣀⣿⣿⣿⠃⠀⠀⠀⠀⠀
 * ⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⡈⠀⠄⠀⠠⣀⠀⡤⣿⣯⠌⠠⠠⠀⠒⡷⡷⣿⡿⠳⠀⠀⢀⣬⣿⣿⣿⣿⣿⣿⣿⣿⣯⢌⣰⢟⠀⠀⠀⠀⢀⣿⣿⣿⣿⣿⣿⠗⠂⣨⣿⣿⣿⣿⠏⠀⠀⠀⠀⠀
 * ⣿⣿⣿⡿⣿⣿⣿⣿⣿⣿⣿⣯⢌⢌⢈⣨⣨⢀⣿⣿⣿⣮⣬⠌⠎⠀⠀⠀⠀⠀⢀⣬⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣯⣎⠀⠀⠀⣼⣿⣿⣿⣿⠗⡿⢀⣾⣿⠿⣿⣿⡿⠁⠀⠀⠀⠀⠀
 * ⣿⣿⣿⠇⡰⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠏⣼⣿⣿⣿⣿⣿⣿⣍⠀⠈⣌⢀⠌⣾⢓⠗⣿⣿⣿⣿⣿⣿⣿⣿⡷⣷⠷⣿⡿⠁⠀⠀⣼⣿⣿⣿⡽⢃⢸⣭⣿⣿⣿⣏⣱⣿⢉⠎⠲⠀⠀⠀⠀
 * ⣷⣿⣟⠀⢀⣰⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⣿⣿⣿⣿⡳⣿⣿⣿⠃⠁⠑⡰⣷⣿⣾⠀⠐⠀⣱⣿⠿⠁⡾⠷⠀⠀⡠⠓⠀⠀⢈⠀⡱⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠄⣿⣿⠏⠀⠀⠀⠀⠀
 * ⣼⣿⣿⠀⠀⣰⣿⣿⣿⣿⣿⣿⡿⣿⣿⣿⠀⠷⣽⣿⣿⢎⠃⣱⣿⢎⢈⣈⢈⠀⠀⠑⠀⠀⢀⠀⢓⢀⠀⢈⠈⠀⢀⢈⠀⠐⣯⣿⠏⠀⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⣼⣿⠏⠂⠀⠀⠀⠀
 * ⣿⣿⣿⣏⠈⡰⣿⣿⣿⣿⣿⣿⠀⣿⣿⡿⣰⣀⠿⣿⣿⣿⣏⠘⣳⣿⣿⣿⣿⣮⣜⣬⣮⣞⣿⣯⣿⣿⣏⣿⣿⣿⣾⣿⣿⣿⣿⣿⣯⠀⣰⣿⣿⣿⣿⣿⣿⣿⣿⠗⠃⠀⣿⣿⠏⠀⠀⠀⠀⠀
 * ⣱⣿⣿⣿⣿⣮⠽⡳⣿⣿⣿⠏⠀⣿⣿⣯⢘⣰⣁⣇⠿⣼⣿⣿⠞⠱⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⣰⣿⣿⣿⣿⠀⡰⠓⠑⠀⠀⣼⣿⣿⠃⠀⠀⠀⠀⠀
 * ⢰⣿⣿⣿⣿⣿⠏⣳⣿⣿⣿⣿⠀⣷⣿⣿⣿⠀⠸⣰⠀⢟⣱⣿⣿⣎⣸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠀⣿⣿⣿⣿⠏⠀⠀⠀⠀⡌⠀⣰⣿⠃⠀⠀⠀⠀⠀⠀
 * ⣰⣿⣿⡿⡾⣷⣯⣰⣿⣿⣿⣿⣎⣸⣿⣿⣿⣯⠏⣰⠀⠏⣼⣿⣷⣷⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⡷⠓⠀⣀⣿⣿⣿⠗⠀⠀⣈⠮⠳⠉⠀⣼⠃⠀⠀⠀⠀⠀⠀⠀
 * ⣰⣿⣿⠎⠀⡰⣿⣟⣷⣿⣿⣿⣿⠟⡰⣳⣿⣿⣿⣾⣈⠏⣳⠳⡰⠰⡷⣳⠗⠷⡷⣿⣿⣿⣿⣿⣿⣿⠷⡷⡷⠳⠳⠳⠑⠁⠀⠀⣨⣿⣿⣿⠏⠂⣠⡴⠓⠀⠀⠀⠴⠁⠀⠀⠀⠀⠀⠀
 * TODO: delete this function when this fix reaches stable version of tendermint⠀⠀
 */
func setupProxy(targetAddr string, logger *zap.Logger) (string, error) {
	targetUrl, err := url.Parse(targetAddr)
	if err != nil {
		return "", fmt.Errorf("%s is not a valid url: %w", targetAddr, err)
	}

	if targetUrl.Scheme == "tcp" {
		targetUrl.Scheme = "http"
	}
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		request.Host = targetUrl.Host
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to bind to random port on 127.0.0.1: %w", err)
	}

	proxyAddr := fmt.Sprintf("http://%s", listener.Addr().String())
	go func() {
		err := http.Serve(listener, proxy)
		if err != nil {
			logger.Fatal("tendermint-workaround reverse proxy has failed", zap.Error(err))
		}
	}()

	logger.Warn("set up tendermint-workaround proxy",
		zap.String("target", targetAddr),
		zap.String("listen", proxyAddr),
	)
	return proxyAddr, nil
}
