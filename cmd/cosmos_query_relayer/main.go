package main

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/event_subscriber"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"
	"log"
)

// TODO: logger configuration

func main() {
	fmt.Println("cosmos-query-relayer starts...")
	ctx := context.Background()
	addr := "tcp://public-node.terra.dev:26657"
	//addr = "tcp://0.0.0.0:26657"
	//testSubscribe(ctx, addr)

	//ccc, logger, homepath := proofer.GetChainConfig()
	querier, err := proofer.NewQueryKeyProofer(addr, "columbus-5")
	fmt.Printf("Got new query proofer")
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

func testSubscribe(ctx context.Context, addr string) {
	err := event_subscriber.SubscribeToTargetChainEventsNative(ctx, addr)

	if err != nil {
		//	TODO
	}
}
