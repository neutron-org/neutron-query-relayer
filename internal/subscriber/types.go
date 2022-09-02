package subscriber

import (
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron/x/interchainqueries/types"
)

// buildMessageKV creates a MessageKV out of the given queryEventMessage.
func buildMessageKV(msg *queryEventMessage) *relay.MessageKV {
	return &relay.MessageKV{
		QueryId: msg.queryId,
		KVKeys:  msg.kvKeys,
	}
}

// buildMessageTX creates a MessageTX out of the given queryEventMessage.
func buildMessageTX(msg *queryEventMessage) *relay.MessageTX {
	return &relay.MessageTX{
		QueryId:            msg.queryId,
		TransactionsFilter: msg.transactionsFilter,
	}
}

// queryEventMessage is a general structure of an interchain query message.
type queryEventMessage struct {
	queryId            uint64
	messageType        types.InterchainQueryType
	kvKeys             types.KVKeys
	transactionsFilter string
}
