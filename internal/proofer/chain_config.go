package proofer

import (
	lens "github.com/strangelove-ventures/lens/client"
	"go.uber.org/zap"
)

// TODO: should take query into account?
func GetChainConfig() (*lens.ChainClientConfig, *zap.Logger, string) {
	logger, _ := zap.NewProduction()
	homepath := "./"
	ccc := lens.ChainClientConfig{
		Key:     "default",
		ChainID: "columbus-5",
		RPCAddr: "http://public-node.terra.dev:26657/",
		//GRPCAddr:       "https://gprc.cosmoshub-4.technofractal.com:443",
		AccountPrefix:  "terra",
		KeyringBackend: "test",
		GasAdjustment:  1.2,
		GasPrices:      "0.01uatom",
		KeyDirectory:   "",
		Debug:          true,
		Timeout:        "20s",
		OutputFormat:   "json",
		SignModeStr:    "direct",
	}

	return &ccc, logger, homepath
}
