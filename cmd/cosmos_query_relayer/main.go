package main

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/relayer"
	"log"

	sub "github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

// TODO: logger configuration

func main() {
	fmt.Println("cosmos-query-relayer starts...")
	ctx := context.Background()
	cfg, err := config.NewCosmosQueryRelayerConfig()
	if err != nil {
		log.Println(err)
	}
	setSDKConfig(cfg)
	testProofs(ctx, cfg)
	//testSubscribeLidoChain(ctx, cfg.LidoChain.RPCAddress)
}

// NOTE: cosmos-sdk sets global values for prefixes when parsing addresses and so on
// Without this some functions just does not work as intended
func setSDKConfig(cfg config.CosmosQueryRelayerConfig) {
	// TODO: we set global prefix for addresses to the lido chain, is it ok?
	sdkCfg := sdk.GetConfig()
	sdkCfg.SetBech32PrefixForAccount(cfg.LidoChain.ChainPrefix, cfg.LidoChain.ChainPrefix+sdk.PrefixPublic)
	//	config.SetBech32PrefixForValidator(yourBech32PrefixValAddr, yourBech32PrefixValPub)
	//	config.SetBech32PrefixForConsensusNode(yourBech32PrefixConsAddr, yourBech32PrefixConsPub)
	//	config.SetPurpose(yourPurpose)
	//	config.SetCoinType(yourCoinType)
	sdkCfg.Seal()
}

func subscribeLidoChain(ctx context.Context, rpcAddress string) {
	onEvent := func(event coretypes.ResultEvent) {
		//TODO: maybe make proofer a class with querier inside and instantiate it here, call GetProof on it?
		fmt.Printf("OnEvent(%+v)", event.Data)
		go relayer.Relayer{}.Proof(event)
	}
	err := sub.Subscribe(ctx, rpcAddress, onEvent, sub.Query)
	if err != nil {
		log.Fatalf("error subscribing to lido chain events: %s", err)
	}
}
