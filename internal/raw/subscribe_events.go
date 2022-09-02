package raw

import (
	"context"
	"fmt"

	rpcclient "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

const websocketPath = "/websocket"

// Subscribe subscribes to a blockchain by the given rpcAddress using websocket and returns a chan
// of events.
//
// WARNING: rpcclient.Subscribe from tendermint can fail to work with some blockchain versions of
// tendermint.
func Subscribe(ctx context.Context, subscriberName string, rpcAddress string, query string) (<-chan coretypes.ResultEvent, error) {
	httpclient, err := rpcclient.New(rpcAddress, websocketPath)
	if err != nil {
		return nil, fmt.Errorf("could not create new rpcclient: %w", err)
	}
	err = httpclient.Start()
	if err != nil {
		return nil, fmt.Errorf("could not start httpclient when subscribing to target chain: %w", err)
	}

	response, err := httpclient.Subscribe(ctx, subscriberName, query)
	if err != nil {
		return nil, fmt.Errorf("could not subscribe to target chain: %w", err)
	}
	return response, nil
}

// SubscribeQuery describes query to filter out events by module and action.
func SubscribeQuery(connectionId string) string {
	return fmt.Sprintf("message.module='%s' AND message.action='%s' AND message.connection_id='%s' AND tm.event='%s'", neutrontypes.ModuleName, neutrontypes.AttributeValueQuery, connectionId, types.EventNewBlockHeader)
}
