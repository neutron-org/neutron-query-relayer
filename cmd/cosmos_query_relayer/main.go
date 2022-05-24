package main

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof/proof_impl"
	"github.com/lidofinance/cosmos-query-relayer/internal/relay"
	"github.com/lidofinance/cosmos-query-relayer/internal/submit"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"log"

	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/raw"
)

func main() {
	fmt.Println("cosmos-query-relayer starts...")

	ctx := context.Background()
	cfg, err := config.NewCosmosQueryRelayerConfig()
	if err != nil {
		log.Fatalf("cannot initialize relayer config: %s", err)
	}

	raw.SetSDKConfig(cfg.LidoChain)

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddress, cfg.TargetChain.Timeout)
	if err != nil {
		log.Fatalf("could not initialize target rpc client: %s", err)
	}

	targetQuerier, err := proof.NewQuerier(targetClient, cfg.TargetChain.ChainID)
	if err != nil {
		log.Fatalf("cannot connect to target chain: %s", err)
	}

	lidoClient, err := raw.NewRPCClient(cfg.LidoChain.RPCAddress, cfg.LidoChain.Timeout)
	if err != nil {
		log.Println(err)
		return
	}

	codec := raw.MakeCodecDefault()
	keybase, err := submit.TestKeybase(cfg.LidoChain.ChainID, cfg.LidoChain.Keyring.Dir)
	if err != nil {
		log.Println(err)
		return
	}

	txSender, err := submit.NewTxSender(ctx, lidoClient, codec.Marshaller, keybase, cfg.LidoChain)
	if err != nil {
		log.Println(err)
		return
	}

	proofSubmitter := submit.NewSubmitterImpl(cfg.LidoChain.Sender, txSender)
	proofFetcher := proof_impl.NewProofer(targetQuerier)
	relayer := relay.NewRelayer(proofFetcher, proofSubmitter, cfg.TargetChain.ChainID, cfg.TargetChain.ChainPrefix, cfg.LidoChain.Sender)

	// NOTE: no parallel processing here. What if proofs or transaction submissions for each event will take too long?
	// Then the proofs will be for past events, but still for last target blockchain state, and that is kinda okay for now
	err = raw.Subscribe(ctx, cfg.LidoChain.RPCAddress, raw.SubscribeQuery(cfg.TargetChain.ChainID), func(event coretypes.ResultEvent) {
		relayer.Proof(ctx, event)
	})
	if err != nil {
		log.Fatalf("error subscribing to lido chain events: %s", err)
	}
}
