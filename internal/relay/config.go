package relay

import (
	"fmt"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"github.com/neutron-org/cosmos-query-relayer/internal/config"
	"go.uber.org/zap"
)

// getChain reads a chain env and adds it to a's chains.
func getChain(logger *zap.Logger, cfg cosmos.CosmosProviderConfig, homepath string, debug bool) (*relayer.Chain, error) {
	prov, err := cfg.NewProvider(
		logger,
		homepath,
		debug,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build ChainProvider for %w", err)
	}

	// Without this hack it doesn't want to work with normal home dir layout for some reason.
	provConcrete, ok := prov.(*cosmos.CosmosProvider)
	if !ok {
		return nil, fmt.Errorf("failed to patch CosmosProvider config (type cast failed)")
	}
	provConcrete.Config.KeyDirectory = homepath

	return relayer.NewChain(logger, prov, debug), nil
}

func GetNeutronChain(logger *zap.Logger, cfg *config.NeutronChainConfig) (*relayer.Chain, error) {
	provCfg := cosmos.CosmosProviderConfig{
		Key:            cfg.SignKeyName,
		ChainID:        cfg.ChainID,
		RPCAddr:        cfg.RPCAddr,
		AccountPrefix:  cfg.AccountPrefix,
		KeyringBackend: cfg.KeyringBackend,
		GasAdjustment:  cfg.GasAdjustment,
		GasPrices:      cfg.GasPrices,
		Debug:          cfg.Debug,
		Timeout:        cfg.Timeout.String(),
		OutputFormat:   cfg.OutputFormat,
		SignModeStr:    cfg.SignModeStr,
	}
	chain, err := getChain(logger, provCfg, cfg.HomeDir, cfg.Debug)
	if err != nil {
		return nil, fmt.Errorf("could not create neutron chain: %w", err)
	}

	return chain, nil
}

func GetTargetChain(logger *zap.Logger, cfg *config.TargetChainConfig) (*relayer.Chain, error) {
	provCfg := cosmos.CosmosProviderConfig{
		Key:            "",
		ChainID:        cfg.ChainID,
		RPCAddr:        cfg.RPCAddr,
		AccountPrefix:  cfg.AccountPrefix,
		KeyringBackend: cfg.KeyringBackend,
		GasAdjustment:  cfg.GasAdjustment,
		GasPrices:      cfg.GasPrices,
		Debug:          cfg.Debug,
		Timeout:        cfg.Timeout.String(),
		OutputFormat:   cfg.OutputFormat,
		SignModeStr:    cfg.SignModeStr,
	}
	chain, err := getChain(logger, provCfg, cfg.HomeDir, cfg.Debug)
	if err != nil {
		return nil, fmt.Errorf("could not create neutron chain: %w", err)
	}

	return chain, nil
}
