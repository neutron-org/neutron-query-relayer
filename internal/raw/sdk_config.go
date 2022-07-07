package raw

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetSDKConfig sets GLOBAL values for prefixes for cosmos-sdk when parsing addresses and so on
// Apparently, there is no way around that for now
// Without this some functions just does not work as intended
func SetSDKConfig(chainPrefix string) {
	cfg := sdk.GetConfig()

	cfg.SetBech32PrefixForAccount(chainPrefix, chainPrefix+sdk.PrefixPublic)
	cfg.SetBech32PrefixForValidator(chainPrefix+sdk.PrefixValidator+sdk.PrefixOperator, chainPrefix+sdk.PrefixValidator+sdk.PrefixOperator+sdk.PrefixPublic)
	cfg.SetBech32PrefixForConsensusNode(chainPrefix+sdk.PrefixValidator+sdk.PrefixConsensus, chainPrefix+sdk.PrefixValidator+sdk.PrefixConsensus+sdk.PrefixPublic)
	cfg.Seal()
}
