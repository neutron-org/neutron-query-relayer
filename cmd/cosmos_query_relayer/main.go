package main

import (
	"context"
	"fmt"
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
	//testSubscribe(ctx)

	ccc, logger, homepath := proofer.GetChainConfig()
	querier, err := proofer.NewQueryKeyProofer(logger, homepath, ccc)
	if err != nil {
		err = fmt.Errorf("error creating new query key proofer: %w", err)
		log.Println(err)
	}
	_, _ = proofs.ProofAllBalances(ctx, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts", querier)
}

func testSubscribe(ctx context.Context) {
	err := event_subscriber.SubscribeToTargetChainEventsNative(ctx)

	if err != nil {
		//	TODO
	}
}

func Proof(ctx context.Context, event coretypes.ResultEvent) {
	// TODO
	queries := proofer.ExpandEvent(event)
	proofer.ProofQueries(ctx, queries)
}
