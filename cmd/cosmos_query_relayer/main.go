package main

import (
	"context"
	"fmt"
	"log"

	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/event_subscriber"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"

	coretypes "github.com/tendermint/tendermint/rpc/core/types"
)

// TODO: logger configuration

func main() {
	fmt.Println("cosmos-query-relayer starts...")
	ctx := context.Background()
	cfg, err := config.NewCosmosQueryRelayerConfig()
	if err != nil {
		log.Println(err)
	}
	//subscribeLidoChain(ctx, cfg.LidoChain.RPCAddress)
	testProofs(ctx, cfg)
}

func subscribeLidoChain(ctx context.Context, addr string) {
	onEvent := func(event coretypes.ResultEvent) error {
		// TODO: proof event
		fmt.Printf("OnEvent(%+v)", event.Data)
		return nil
	}
	err := event_subscriber.SubscribeToTargetChainEventsNative(ctx, addr, onEvent)

	if err != nil {
		//	TODO
	}
}

func testProofs(ctx context.Context, cfg config.CosmosQueryRelayerConfig) {
	querier, err := proofer.NewProofQuerier(cfg.TargetChain.RPCAddress, cfg.TargetChain.ChainID)
	if err != nil {
		err = fmt.Errorf("error creating new query key proofer: %w", err)
		log.Println(err)
	}
	_, _ = proofs.ProofAllBalances(ctx, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts", "uluna", querier)
	//_, err = proofs.ProofAllDelegations(ctx, []string{"terravaloper123gn6j23lmexu0qx5qhmgxgunmjcqsx8gmsyse"}, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts", querier)
	//_, err = proofs.ProofAllDelegations2(ctx, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts", querier)

	if err != nil {
		log.Println(err)
	}
}
