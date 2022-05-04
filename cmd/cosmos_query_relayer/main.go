package main

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"
	"log"

	sub "github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
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
	//subscribeLidoChain(ctx, cfg.LidoChain.RPCAddress)
}

func subscribeLidoChain(ctx context.Context, addr string) {
	onEvent := func(event coretypes.ResultEvent) {
		//TODO: maybe make proofer a class with querier inside and instantiate it here, call GetProof on it?
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
	//_, _ = proofs.ProofAllBalances(ctx, querier, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts", "uluna")
	//_, err = proofs.ProofAllDelegations(ctx, querier, []string{"terravaloper123gn6j23lmexu0qx5qhmgxgunmjcqsx8gmsyse"}, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	//_, err = proofs.ProofAllDelegations2(ctx, querier, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	testTxProof(ctx, cfg, querier)

	if err != nil {
		log.Println(err)
	}
}

func testTxProof(ctx context.Context, cfg config.CosmosQueryRelayerConfig, querier *proofer.ProofQuerier) {
	//hash := "0xE71F89160178AE8A6AC84F6F8810658CEDF4A66FACA27BA2FFFF2DA8539DE4A6"
	//value, err := querier.QueryTxProof(ctx, 0, []byte(hash))
	//var tx cosmostypes.Tx
	//log.Println()

	// https://atomscan.com/terra
	height := int64(7503466)
	proofs.TxCompletedSuccessfullyProof(ctx, querier, height)
}
