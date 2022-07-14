package raw

import (
	"context"
	"fmt"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
)

const websocketPath = "/websocket"

// Subscribe subscribes to target blockchain using websockets
// WARNING: rpcclient.Subscribe from tendermint can fail to work with some blockchain versions of tendermint
func Subscribe(ctx context.Context, subscriberName string, rpcAddress string, query string, onEvent func(event coretypes.ResultEvent)) error {
	httpclient, err := rpcclient.New(rpcAddress, websocketPath)
	if err != nil {
		return fmt.Errorf("could not create new rpcclient: %w", err)
	}
	err = httpclient.Start()
	if err != nil {
		return fmt.Errorf("could not start httpclient when subscribing to target chain: %w", err)
	}

	response, err := httpclient.Subscribe(ctx, subscriberName, query)
	if err != nil {
		return fmt.Errorf("could not subscribe to target chain: %w", err)
	}

	for e := range response {
		onEvent(e)
	}

	return nil
}

// SubscribeQuery describes query to filter out events with correct module, action and zone_id
func SubscribeQuery(zoneId string) string {
	// TODO: fix after zone_id is saved in message
	//return fmt.Sprintf("message.module='%s' AND message.action='%s' AND message.zone_id='%s'", neutrontypes.ModuleName, neutrontypes.AttributeValueQuery, zoneId)
	return fmt.Sprintf("message.module='%s' AND message.action='%s'", neutrontypes.ModuleName, neutrontypes.AttributeValueQuery)
}
