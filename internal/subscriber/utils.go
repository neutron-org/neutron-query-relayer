package subscriber

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	tmhttp "github.com/cometbft/cometbft/rpc/client/http"
	tmtypes "github.com/cometbft/cometbft/rpc/core/types"
	jsonrpcclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/cometbft/cometbft/types"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"go.uber.org/zap"

	restclient "github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

var (
	restClientBasePath = "/"
	rpcWSEndpoint      = "/websocket"
)

// NewRPCClient creates a new tendermint RPC client with timeout.
func NewRPCClient(rpcAddr string, timeout time.Duration) (RpcHttpClient, error) {
	httpClient, err := jsonrpcclient.DefaultHTTPClient(rpcAddr)
	if err != nil {
		return nil, err
	}
	httpClient.Timeout = timeout
	client, err := tmhttp.NewWithClient(rpcAddr, rpcWSEndpoint, httpClient)
	return *client, err
}

// NewRESTClient makes sure that the restAddr is formed correctly and returns a REST query.
func NewRESTClient(restAddr string, timeout time.Duration) (*restclient.HTTPAPIConsole, error) {
	url, err := url.Parse(restAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse restAddr: %w", err)
	}

	httpClient := http.DefaultClient
	httpClient.Timeout = timeout
	transport := httptransport.NewWithClient(url.Host, restClientBasePath, []string{url.Scheme}, httpClient)

	return restclient.New(transport, nil), nil
}

// getNeutronRegisteredQuery retrieves a registered query from Neutron.
func (s *Subscriber) getNeutronRegisteredQuery(ctx context.Context, queryId string) (*neutrontypes.RegisteredQuery, error) {
	res, err := s.restClientQuery.NeutronInterchainQueriesRegisteredQuery(
		&query.NeutronInterchainQueriesRegisteredQueryParams{
			QueryID: &queryId,
			Context: ctx,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get NeutronInterchainqueriesRegisteredQuery: %w", err)
	}
	neutronQuery, err := res.GetPayload().RegisteredQuery.ToNeutronRegisteredQuery()
	if err != nil {
		return nil, fmt.Errorf("failed to get neutronQueryFromRestQuery: %w", err)
	}
	return neutronQuery, nil
}

// getNeutronRegisteredQueries retrieves the list of registered queries filtered by owner, connection, and query type.
func (s *Subscriber) getNeutronRegisteredQueries(ctx context.Context) (map[string]*neutrontypes.RegisteredQuery, error) {
	var out = map[string]*neutrontypes.RegisteredQuery{}
	var pageKey *strfmt.Base64
	for {
		res, err := s.restClientQuery.NeutronInterchainQueriesRegisteredQueries(
			&query.NeutronInterchainQueriesRegisteredQueriesParams{
				Owners:        s.registry.GetAddresses(),
				ConnectionID:  &s.connectionID,
				Context:       ctx,
				PaginationKey: pageKey,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get NeutronInterchainqueriesRegisteredQueries: %w", err)
		}

		payload := res.GetPayload()

		for _, restQuery := range payload.RegisteredQueries {
			neutronQuery, err := restQuery.ToNeutronRegisteredQuery()
			if err != nil {
				return nil, fmt.Errorf("failed to cast ToNeutronRegisteredQuery: %w", err)
			}

			if !s.isWatchedMsgType(neutronQuery.QueryType) {
				continue
			}
			out[restQuery.ID] = neutronQuery
		}
		if payload.Pagination != nil && payload.Pagination.NextKey.String() != "" {
			pageKey = &payload.Pagination.NextKey
		} else {
			break
		}
	}
	s.logger.Debug("total queries fetched", zap.Int("queries number", len(out)))

	return out, nil
}

// checkEvents verifies that 1. there is N events associated with the connection id that we are
// interested in, 2. there is a matching number of other query-specific event attributes.
func (s *Subscriber) checkEvents(event tmtypes.ResultEvent) (bool, error) {
	events := event.Events

	icqEventsCount := len(events[connectionIdAttr])
	if icqEventsCount == 0 {
		s.logger.Debug("no connection id attributes received", zap.Any("events", events))
		return false, nil
	}

	if len(events[kvKeyAttr]) != icqEventsCount ||
		len(events[transactionsFilterAttr]) != icqEventsCount ||
		len(events[queryIdAttr]) != icqEventsCount ||
		len(events[typeAttr]) != icqEventsCount {
		return false, fmt.Errorf("events attributes length does not match for events=%v", events)
	}

	return true, nil
}

// subscriberName returns the subscriber name.
// Note: it doesn't matter what we return here because Tendermint will override it with
// remote IP anyway.
func (s *Subscriber) subscriberName() string {
	return "neutron-rpcClient"
}

// getQueryUpdatedSubscription returns a Query to filter out interchain "query_updated" events.
func (s *Subscriber) getQueryUpdatedSubscription() string {
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s'",
		connectionIdAttr, s.connectionID,
		moduleAttr, neutrontypes.ModuleName,
		actionAttr, neutrontypes.AttributeValueQueryUpdated,
	)
}

// getQueryRemovedSubscription returns a Query to filter out interchain "query_removed" events.
func (s *Subscriber) getQueryRemovedSubscription() string {
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s'",
		connectionIdAttr, s.connectionID,
		moduleAttr, neutrontypes.ModuleName,
		actionAttr, neutrontypes.AttributeValueQueryRemoved,
	)
}

// getNewBlockHeaderSubscription returns a Query to filter out interchain "NewBlockHeader" events.
func (s *Subscriber) getNewBlockHeaderSubscription() string {
	return fmt.Sprintf("%s='%s'",
		eventAttr, types.EventNewBlockHeader,
	)
}

// isWatchedMsgType returns true if the given message type was added to the subscriber's watched
// ActiveQuery types list.
func (s *Subscriber) isWatchedMsgType(msgType string) bool {
	_, ex := s.watchedTypes[neutrontypes.InterchainQueryType(msgType)]
	return ex
}

// isWatchedAddress returns true if the address is within the registry watched addresses or there
// are no registry watched addresses configured for the subscriber meaning all addresses are watched.
func (s *Subscriber) isWatchedAddress(address string) bool {
	return s.registry.IsEmpty() || s.registry.Contains(address)
}
