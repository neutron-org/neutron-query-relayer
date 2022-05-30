package raw

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetSDKConfig sets GLOBAL values for prefixes for cosmos-sdk when parsing addresses and so on
// Apparently, there is no way around that for now
// Without this some functions just does not work as intended
func SetSDKConfig(chainPrefix string) {
	sdkCfg := sdk.GetConfig()
	sdkCfg.SetBech32PrefixForAccount(chainPrefix, chainPrefix+sdk.PrefixPublic)
	sdkCfg.Seal()
}
