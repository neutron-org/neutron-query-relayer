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

	targetClient, err := sub.NewRPCClient(cfg.TargetChain.RPCAddress, cfg.TargetChain.Timeout)
	if err != nil {
		log.Fatalf("could not initialize target rpc client: %s", err)
	}

	targetQuerier, err := proofer.NewProofQuerier(targetClient, cfg.TargetChain.ChainID)
	if err != nil {
		log.Fatalf("cannot connect to target chain: %s", err)
	}

	//testProofs(ctx, cfg)

	lidoClient, err := sub.NewRPCClient(cfg.LidoChain.RPCAddress, cfg.LidoChain.Timeout)
	if err != nil {
		log.Println(err)
		return
	}

	codec := sub.MakeCodecConfig()
	keybase, err := sub.TestKeybase(cfg.LidoChain.ChainID, cfg.LidoChain.Keyring.Dir, codec)
	if err != nil {
		log.Println(err)
		return
	}
	txSubmitter, err := sub.NewTxSubmitter(ctx, lidoClient, codec, keybase, cfg)
	if err != nil {
		log.Println(err)
		return
	}
	proofSubmitter := submitter.NewProofSubmitter(txSubmitter)

	relayer := relay.NewRelayer(targetQuerier, proofSubmitter, cfg.TargetChain.ChainPrefix, cfg.LidoChain.Sender)

	// NOTE: no parallel processing here. What if proofs or transaction submissions for each event will take too long?
	// Then the proofs will be for past events, but still for last target blockchain state, and that is kinda okay for now
	err = sub.Subscribe(ctx, cfg.LidoChain.RPCAddress, sub.SubscribeQuery(cfg.TargetChain.ChainID), func(event coretypes.ResultEvent) {
		relayer.Proof(ctx, event)
	})
	if err != nil {
		log.Fatalf("error subscribing to lido chain events: %s", err)
	}
}
