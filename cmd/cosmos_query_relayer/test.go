package main

import (
	"context"
	"fmt"
	sub "github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"log"
)

func testSubscribeLidoChain(ctx context.Context, addr string) {
	onEvent := func(event coretypes.ResultEvent) {
		//TODO: maybe make proofer a class with querier inside and instantiate it here, call GetProof on it?
		fmt.Printf("OnEvent(%+v)", event.Data)
	}
	err := sub.Subscribe(ctx, addr, onEvent, sub.Query)
	if err != nil {
		log.Fatalf("error subscribing to lido chain events: %s", err)
	}
}

func testProofs(ctx context.Context, cfg config.CosmosQueryRelayerConfig) {
	querier, err := proofer.NewProofQuerier(cfg.TargetChain.Timeout, cfg.TargetChain.RPCAddress, cfg.TargetChain.ChainID)
	if err != nil {
		err = fmt.Errorf("error creating new query key proofer: %w", err)
		log.Println(err)
	}
	//_, err = proofs.ProofAllBalances(ctx, querier, "terra", "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts", "uluna")
	//_, err = proofs.ProofAllBalances(ctx, querier, "cosmos", "cosmos1jp6xu6hjqap38wk72wk2lmaxvfqswupjamahpl", "stake")
	//_, err = proofs.ProofAllDelegations(ctx, querier, []string{"terravaloper123gn6j23lmexu0qx5qhmgxgunmjcqsx8gmsyse"}, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	//_, err = proofs.ProofAllDelegations2(ctx, querier, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	//err = proofs.ProofRewards(ctx, querier, "terra", "terravaloper123gn6j23lmexu0qx5qhmgxgunmjcqsx8gmsyse", "terra1qqqz0ddedgke63z8xm8pqrujnxl0f9zdvus7yg", 0)

	// {"transfer.recipient": interchain_account}
	//recipientAddress := "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts"
	//recipientAddress := "terra1ncjg4a59x2pgvqy9qjyqprlj8lrwshm0wleht5"
	//query := fmt.Sprintf("message.recipient='%s'", recipientAddress)

	// Test on local terra
	//query := fmt.Sprintf("message.sender='%s'", "terra1x46rqay4d3cssq8gxxvqz8xt6nwlz4td20k38v")
	//query := "tm.event = 'Tx'"
	//query := "tx.height=3469"

	query := fmt.Sprintf("transfer.recipient='%s'", "terra17lmam6zguazs5q5u6z5mmx76uj63gldnse2pdp")
	_, err = proofs.ProofTransactions(ctx, querier, query)

	//testTxProof(ctx, cfg, querier)

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
	indexInBlock := uint32(0)
	proof, _ := proofs.TxCompletedSuccessfullyProof(ctx, querier, height, indexInBlock)

	results, _ := querier.Client.BlockResults(ctx, &height)
	err := proofs.VerifyProof(results, *proof, indexInBlock)

	if err == nil {
		log.Println("Verification passed")
	} else {
		log.Println("Verification failed")
	}
}
