package chain

import (
	"context"
	"fmt"
	rpcclient "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

// Subscribe subscribes to target blockchain using websockets
// WARNING: rpcclient.Subscribe from tendermint can fail to work with some blockchain versions of tendermint
func Subscribe(ctx context.Context, rpcAddress string, query string, onEvent func(event coretypes.ResultEvent)) error {
	httpclient, err := rpcclient.New(rpcAddress)
	if err != nil {
		return fmt.Errorf("could not create new rpcclient: %w", err)
	}
	err = httpclient.Start()
	if err != nil {
		return fmt.Errorf("could not start httpclient when subscribing to target chain: %w", err)
	}

	response, err := httpclient.Subscribe(ctx, "cosmos-query-relayer", query)
	if err != nil {
		return fmt.Errorf("could not subscribe to target chain: %w", err)
	}

	for e := range response {
		onEvent(e)
	}

	return nil
}

func SubscribeQuery(zoneId string) string {
	return fmt.Sprintf("message.module='%s' AND message.action='%s' AND message.zone_id='%s'", "interchainqueries", "query", zoneId)
}
