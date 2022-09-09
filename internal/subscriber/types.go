package subscriber

import (
	"github.com/neutron-org/neutron/x/interchainqueries/types"
)

// queryEventMessage is a general structure of an interchain ActiveQuery message.
type queryEventMessage struct {
	queryId            uint64
	messageType        types.InterchainQueryType
	kvKeys             types.KVKeys
	transactionsFilter string
}
