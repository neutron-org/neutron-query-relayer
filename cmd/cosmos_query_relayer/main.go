package main

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/event_subscriber"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"log"
)

// TODO: logger configuration

func main() {
	fmt.Println("cosmos-query-relayer starts...")
	ctx := context.Background()
	cfg, err := config.NewCosmosQueryRelayerConfig()
	if err != nil {
		log.Println(err)
	}
	subscribeLidoChain(ctx, cfg.LidoChain.RPCAddress)
	querier, err := proofer.NewQueryKeyProofer(cfg.TargetChain.RPCAddress, cfg.TargetChain.ChainID)
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

func subscribeLidoChain(ctx context.Context, addr string) {
	onEvent := func(event coretypes.ResultEvent) {
		fmt.Printf("OnEvent(%+v)", event.Data)
	}
	err := event_subscriber.SubscribeToTargetChainEventsNative(ctx, addr, onEvent)

	if err != nil {
		//	TODO
	}
}
