package subscriber

import (
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron/x/interchainqueries/types"
)

// buildMessageKV creates a MessageKV out of the given queryEventMessage.
func buildMessageKV(queryID uint64, kvKeys types.KVKeys) *relay.MessageKV {
	return &relay.MessageKV{
		QueryId: queryID,
		KVKeys:  kvKeys,
	}
}

// buildMessageTX creates a MessageTX out of the given queryEventMessage.
func buildMessageTX(queryID uint64, transactionsFilter string) *relay.MessageTX {
	return &relay.MessageTX{
		QueryId:            queryID,
		TransactionsFilter: transactionsFilter,
	}
}

// queryEventMessage is a general structure of an interchain ActiveQuery message.
type queryEventMessage struct {
	queryId            uint64
	messageType        types.InterchainQueryType
	kvKeys             types.KVKeys
	transactionsFilter string
}
