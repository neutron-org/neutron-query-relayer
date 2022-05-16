package chain

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
)

// NOTE: cosmos-sdk sets global values for prefixes when parsing addresses and so on
// Without this some functions just does not work as intended
func SetSDKConfig(cfg config.CosmosQueryRelayerConfig) {
	// TODO: we set global prefix for addresses to the lido chain, is it ok?
	sdkCfg := sdk.GetConfig()
	sdkCfg.SetBech32PrefixForAccount(cfg.LidoChain.ChainPrefix, cfg.LidoChain.ChainPrefix+sdk.PrefixPublic)
	//	config.SetBech32PrefixForValidator(yourBech32PrefixValAddr, yourBech32PrefixValPub)
	//	config.SetBech32PrefixForConsensusNode(yourBech32PrefixConsAddr, yourBech32PrefixConsPub)
	//	config.SetPurpose(yourPurpose)
	//	config.SetCoinType(yourCoinType)
	sdkCfg.Seal()
}
