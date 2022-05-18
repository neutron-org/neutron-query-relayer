package main

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/relay"
	"github.com/lidofinance/cosmos-query-relayer/internal/submitter"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"log"

	sub "github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
)

// TODO: logger configuration
// TODO: monitoring

func main() {
	fmt.Println("cosmos-query-relayer starts...")

	ctx := context.Background()
	cfg, err := config.NewCosmosQueryRelayerConfig()
	if err != nil {
		log.Fatalf("cannot initialize relayer config: %s", err)
	}

	sub.SetSDKConfig(cfg)
	querier, err := proofer.NewProofQuerier(cfg.TargetChain.Timeout, cfg.TargetChain.RPCAddress, cfg.TargetChain.ChainID)
	if err != nil {
		log.Fatalf("cannot connect to target chain: %s", err)
	}

	//testProofs(ctx, cfg)

	lidoRPCClient, err := proofer.NewRPCClient(cfg.LidoChain.RPCAddress, cfg.LidoChain.Timeout)
	if err != nil {
		log.Println(err)
		return
	}
	// TODO: pick key backend: https://docs.cosmos.network/master/run-node/keyring.html
	codec := sub.MakeCodecDefault()
	keybase, err := sub.TestKeybase(cfg.LidoChain.ChainID, cfg.LidoChain.Keyring.Dir, codec)
	if err != nil {
		log.Println(err)
		return
	}
	txSubmitter, err := sub.NewTxSubmitter(ctx, lidoRPCClient, cfg.LidoChain.ChainID, codec, cfg.LidoChain.GasAdjustment, cfg.LidoChain.Keyring.GasPrices, cfg.LidoChain.ChainPrefix, keybase, cfg.LidoChain.Keyring.SignKeyName)
	if err != nil {
		log.Println(err)
		return
	}
	submit := submitter.NewProofSubmitter(txSubmitter)

	relayer := relay.NewRelayer(querier, submit, cfg.TargetChain.ChainPrefix, cfg.LidoChain.Sender)

	err = sub.Subscribe(ctx, cfg.LidoChain.RPCAddress, sub.SubscribeQuery(cfg.TargetChain.ChainID), func(event coretypes.ResultEvent) {
		relayer.Proof(ctx, event)
	})
	if err != nil {
		log.Fatalf("error subscribing to lido chain events: %s", err)
	}
}
