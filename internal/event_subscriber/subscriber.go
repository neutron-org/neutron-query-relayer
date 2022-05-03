package event_subscriber

import (
	"context"
	"log"

	rpcclient "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

// SubscribeToTargetChainEventsNative subscribes to target blockchain using websockets
// WARNING: rpcclient from tendermint can fail to work with some blockchain versions of tendermint
func SubscribeToTargetChainEventsNative(ctx context.Context, addr string, onEvent func(event coretypes.ResultEvent) error) error {
	httpclient, err := rpcclient.New(addr)
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

	// NOTE: usually:
	// query = 'module_name.action.field=X'
	query := "tm.event='NewBlock'"
	//query := tmquery.MustParse(fmt.Sprintf("message.module='%s'", "interchainqueries")) // todo: use types.ModuleName
	response, err := httpclient.Subscribe(ctx, "test-client", query)
	if err != nil {
		// handle error
		log.Fatalln(err)
	}

	for e := range response {
		err := onEvent(e)
		if err != nil {
			// TODO: handle error
		}
	}

	return nil
}
