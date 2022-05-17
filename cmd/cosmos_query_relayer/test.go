package main

import (
	"context"
	"fmt"
	sub "github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"
	"github.com/lidofinance/cosmos-query-relayer/internal/submitter"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"log"
)

func testSubscribeLidoChain(ctx context.Context, addr string, query string) {
	onEvent := func(event coretypes.ResultEvent) {
		//TODO: maybe make proofer a class with querier inside and instantiate it here, call GetProof on it?
		fmt.Printf("OnEvent:\n%+v\n\n\n", event.Data)
		fmt.Printf("\n\nInner events:\n%+v\n\n", event.Events)
	}
	err := sub.Subscribe(ctx, addr, query, onEvent)
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
	//_, err = proofs.GetBalance(ctx, querier, "terra", "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts", "uluna")
	//_, err = proofs.GetBalance(ctx, querier, "cosmos", "cosmos1jp6xu6hjqap38wk72wk2lmaxvfqswupjamahpl", "stake")
	//_, err = proofs.ProofAllDelegations(ctx, querier, "terra", []string{"terravaloper123gn6j23lmexu0qx5qhmgxgunmjcqsx8gmsyse"}, "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	//_, err = proofs.ProofAllDelegations2(ctx, querier, "terra", "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts")
	//err = proofs.ProofRewards(ctx, querier, "terra", "terravaloper123gn6j23lmexu0qx5qhmgxgunmjcqsx8gmsyse", "terra1qqqz0ddedgke63z8xm8pqrujnxl0f9zdvus7yg", 0)

	//addresses := []string{"terravaloper123gn6j23lmexu0qx5qhmgxgunmjcqsx8gmsyse", "terravaloper15zcjduavxc5mkp8qcqs9eyhwlqwdlrzy6jln3m", "terravaloper1v5hrqlv8dqgzvy0pwzqzg0gxy899rm4kdur03x", "terravaloper1kprce6kc08a6l03gzzh99hfpazfjeczfpzkkau", "terravaloper1c9ye54e3pzwm3e0zpdlel6pnavrj9qqvq89r3r", "terravaloper144l7c3uph5a7h62xd8u5et3rqvj3dqtvvka2fu", "terravaloper1542ek7muegmm806akl0lam5vlqlph7spflfcun", "terravaloper1sym8gyehrdsm03vdc44rg9sflg8zeuqwfzavhx", "terravaloper1khfcg09plqw84jxy5e7fj6ag4s2r9wqsgm7k94", "terravaloper15urq2dtp9qce4fyc85m6upwm9xul30496sgk37", "terravaloper1alpf6snw2d76kkwjv3dp4l7pcl6cn9uyt0tcj9", "terravaloper1nwrksgv2vuadma8ygs8rhwffu2ygk4j24w2mku", "terravaloper175hhkyxmkp8hf2zrzka7cnn7lk6mudtv4uuu64", "terravaloper13g7z3qq6f00qww3u4mpcs3xw5jhqwraswraapc", "terravaloper1jkqr2vfg4krfd4zwmsf7elfj07cjuzss30ux8g", "terravaloper15cupwhpnxhgylxa8n4ufyvux05xu864jcv0tsw"}
	//for _, address := range addresses {
	//	err = proofs.ProofRewards(ctx, querier, "terra", address, "terra1qqqz0ddedgke63z8xm8pqrujnxl0f9zdvus7yg", 0)
	//}

	// {"transfer.recipient": interchain_account}
	//recipientAddress := "terra1mtwph2juhj0rvjz7dy92gvl6xvukaxu8rfv8ts"
	//recipientAddress := "terra1ncjg4a59x2pgvqy9qjyqprlj8lrwshm0wleht5"
	//query := fmt.Sprintf("message.recipient='%s'", recipientAddress)

	// Test on local terra
	//query := fmt.Sprintf("message.sender='%s'", "terra1x46rqay4d3cssq8gxxvqz8xt6nwlz4td20k38v")
	//query := "tm.event = 'Tx'"
	//query := "tx.height=3469"

	//query := fmt.Sprintf("transfer.recipient='%s'", "terra17lmam6zguazs5q5u6z5mmx76uj63gldnse2pdp")
	//_, err = proofs.ProofTransactions(ctx, querier, query)

	//testTxProof(ctx, cfg, querier)

	_, _, err = proofs.GetSupply(ctx, querier, "uluna")

	if err != nil {
		log.Println(err)
	}

	//testTxSubmit(ctx, cfg)
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

func testTxSubmit(ctx context.Context, cfg config.CosmosQueryRelayerConfig) {
	lidoRPCClient, err := proofer.NewRPCClient(cfg.LidoChain.RPCAddress, cfg.LidoChain.Timeout)
	if err != nil {
		log.Println(err)
	}
	// TODO: pick key backend: https://docs.cosmos.network/master/run-node/keyring.html
	codec := sub.MakeCodecDefault()
	keybase, _ := sub.TestKeybase(cfg.LidoChain.ChainID, "test", cfg.LidoChain.Keyring.Dir, codec)
	txSubmitter, err := sub.NewTxSubmitter(ctx, lidoRPCClient, cfg.LidoChain.ChainID, codec, cfg.LidoChain.GasAdjustment, cfg.LidoChain.Keyring.GasPrices, cfg.LidoChain.ChainPrefix, keybase)
	if err != nil {
		log.Println(err)
		return
	}
	proofSubmitter := submitter.NewProofSubmitter(txSubmitter)

	err = proofSubmitter.SendCoins("terra17lmam6zguazs5q5u6z5mmx76uj63gldnse2pdp", "terra1x46rqay4d3cssq8gxxvqz8xt6nwlz4td20k38v")
	if err != nil {
		log.Println(err)
	}
}
