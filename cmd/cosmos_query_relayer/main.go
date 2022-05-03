package main

import (
	"context"
	"fmt"
	"log"

	sub "github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"
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
	testProofs(ctx, cfg)
	subscribeLidoChain(ctx, cfg.LidoChain.RPCAddress)
}

func subscribeLidoChain(ctx context.Context, addr string) {
	onEvent := func(event coretypes.ResultEvent) {
		fmt.Printf("OnEvent(%+v)", event.Data)
		go proofer.GetProof(event)
	}
	err := sub.Subscribe(ctx, addr, onEvent, sub.Query)
	if err != nil {
		log.Fatalf("error subscribing to lido chain events: %s", err)
	}
}

func testProofs(ctx context.Context, cfg config.CosmosQueryRelayerConfig) {
	querier, err := proofer.NewProofQuerier(cfg.TargetChain.RPCAddress, cfg.TargetChain.ChainID)
	if err != nil {
		err = fmt.Errorf("error creating new query key proofer: %w", err)
		log.Println(err)
	}
	_, _ = proofs.ProofAllBalances(ctx, querier, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts", "uluna")
	_, err = proofs.ProofAllDelegations(ctx, querier, []string{"terravaloper123gn6j23lmexu0qx5qhmgxgunmjcqsx8gmsyse"}, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	_, err = proofs.ProofAllDelegations2(ctx, querier, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")

	if err != nil {
		log.Println(err)
	}
}
