package proofer

import (
	lens "github.com/strangelove-ventures/lens/client"
	"go.uber.org/zap"
)

// TODO: should take query into account?
func GetChainConfig(query Query) (*lens.ChainClientConfig, *zap.Logger, string) {
	logger, _ := zap.NewProduction()
	homepath := "./"
	ccc := lens.GetCosmosHubConfig("keys", false)

	return ccc, logger, homepath
}
