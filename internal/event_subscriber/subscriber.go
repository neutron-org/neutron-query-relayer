package event_subscriber

import (
	"context"
	"fmt"
	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"
	"log"
)

// query = 'module_name.action.field=X'
//query_type := "x/staking/GetAllDelegations"
//query := tmquery.MustParse(fmt.Sprintf("message.module='%s'", "interchainqueries")) // todo: use types.ModuleName
func SubscribeToTargetChainEventsNative(ctx context.Context) error {
	remote := "http://public-node.terra.dev:26657"
	httpclient, err := rpcclienthttp.New(remote, "/websocket")
	if err != nil {
		//	TODO
		log.Fatalln(err)
	}
	err = httpclient.Start()
	if err != nil {
		//	TODO
		log.Fatalln(err)
	}

	//defer httpclient.Stop()
	//TODO: what does it mean?
	//ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	//defer cancel()

	query := "tm.event='NewBlock'"
	response, err := httpclient.Subscribe(ctx, "test-client", query)
	if err != nil {
		// handle error
		log.Fatalln(err)
	}

	//ResultEvent
	for e := range response {
		fmt.Printf("got %+v", e.Data)
		//e.Events
	}

	return nil
}
