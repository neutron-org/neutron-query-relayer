package relay

import (
	"context"

	"github.com/neutron-org/neutron/v4/x/interchainqueries/types"
	neutrontypes "github.com/neutron-org/neutron/v4/x/interchainqueries/types"
)

// Subscriber is an interface that subscribes to Neutron and provides chain data in real time.
type Subscriber interface {
	// Subscribe starts sending neutrontypes.RegisteredQuery values to the tasks channel when
	// respective queries need to be updated.
	Subscribe(ctx context.Context, tasks chan neutrontypes.RegisteredQuery) error
}

// MessageKV contains params of a KV interchain query.
type MessageKV struct {
	// QueryId is the ID of the query.
	QueryId uint64
	// KVKeys is the query parameter that describes keys list to be retrieved.
	KVKeys types.KVKeys
}

// MessageTX contains params of a TX interchain query.
type MessageTX struct {
	// QueryId is the ID of the query.
	QueryId uint64
	// TransactionsFilter is the query parameter that describes conditions for transactions search.
	TransactionsFilter string
}
