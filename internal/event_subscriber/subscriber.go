package event_subscriber

import (
	"context"
	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"log"
)

// query = 'module_name.action.field=X'
//query_type := "x/staking/GetAllDelegations"
//query := tmquery.MustParse(fmt.Sprintf("message.module='%s'", "interchainqueries")) // todo: use types.ModuleName
func SubscribeToTargetChainEventsNative(ctx context.Context, addr string, onEvent func(event coretypes.ResultEvent)) error {
	httpclient, err := rpcclienthttp.New(addr, "/websocket")
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

	for e := range response {
		onEvent(e)
	}

	return nil
}
