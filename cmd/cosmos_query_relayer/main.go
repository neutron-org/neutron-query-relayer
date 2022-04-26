package main

import (
	"context"
	"fmt"
	lens "github.com/strangelove-ventures/lens/client"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	"go.uber.org/zap"
	"os"
)

// TODO: logger configuration

func main() {
	fmt.Println("cosmos-query-relayer starts...")
	ctx := context.Background()
	// config, err := config.NewCosmosQueryRelayerConfig()
	SubscribeToTargetChainEvents(ctx)
}

//ccc := lens.ChainClientConfig{
//Key:     "default",
//ChainID: "testnet",
//RPCAddr: "http://public-node.terra.dev:26657",
////GRPCAddr:       "http://gprc.localhost:26657",
//AccountPrefix:  "terra", // can import from interchain queries module
//KeyringBackend: "test",
//GasAdjustment:  1.2,
//GasPrices:      "0.01uatom",
//KeyDirectory:   "./keys",
//Debug:          false,
//Timeout:        "20s",
//OutputFormat:   "json",
//SignModeStr:    "direct",
//}

func SubscribeToTargetChainEvents(ctx context.Context) error {
	//wg := &sync.WaitGroup{}

	logger, _ := zap.NewProduction()
	homepath := "./"
	ccc := lens.ChainClientConfig{
		Key:     "default",
		ChainID: "testnet",
		RPCAddr: "http://localhost:26657",
		//GRPCAddr:       "http://gprc.localhost:26657",
		AccountPrefix:  "cosmos", // can import from interchain queries module
		KeyringBackend: "test",
		GasAdjustment:  1.2,
		GasPrices:      "0.01uatom",
		KeyDirectory:   "./keys",
		Debug:          false,
		Timeout:        "20s",
		OutputFormat:   "json",
		SignModeStr:    "direct",
	}
	//ccc := lens.GetCosmosHubConfig("./keys", false)
	//ccc := lens.GetOsmosisConfig("./keys", false)
	client, err := lens.NewChainClient(logger, &ccc, homepath, os.Stdin, os.Stdout)
	if err != nil {
		// TODO
		return err
	}
	err = client.RPCClient.Start()
	if err != nil {
		fmt.Println(err)
		// TODO
		return err
	}
	// query = 'module_name.action.field=X'
	//query_type := "x/staking/GetAllDelegations"
	//query := tmquery.MustParse(fmt.Sprintf("message.module='%s'", "interchainqueries")) // todo: use types.ModuleName
	query := tmquery.MustParse(fmt.Sprintf("tm.event='NewBlock'"))
	ch, err := client.RPCClient.Subscribe(ctx, client.Config.ChainID+"-icq", query.String())

	fmt.Println("waiting for events...")
	for event := range ch {
		fmt.Printf("\nincoming event: %+v", event)
	}
	//wg.Add(1)

	//wg.Wait()

	return nil
}
