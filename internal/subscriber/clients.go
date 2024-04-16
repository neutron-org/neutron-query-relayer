package subscriber

import (
	"context"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
)

type RpcHttpClient interface {
	Start() error
	Subscribe(ctx context.Context, subscriber string, query string, outCapacity ...int) (out <-chan ctypes.ResultEvent, err error)
	Status(ctx context.Context) (*ctypes.ResultStatus, error)
	Unsubscribe(ctx context.Context, subscriber, query string) error
}

type RestHttpQuery interface {
	NeutronInterchainQueriesRegisteredQuery(params *query.NeutronInterchainQueriesRegisteredQueryParams, opts ...query.ClientOption) (*query.NeutronInterchainQueriesRegisteredQueryOK, error)
	NeutronInterchainQueriesRegisteredQueries(params *query.NeutronInterchainQueriesRegisteredQueriesParams, opts ...query.ClientOption) (*query.NeutronInterchainQueriesRegisteredQueriesOK, error)
}
