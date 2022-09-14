package subscriber

import (
	"context"
	"fmt"
	"net/url"

	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"

	restclient "github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// newRESTClient makes sure that the restAddr is formed correctly and returns a REST query.
func newRESTClient(restAddr string) (*restclient.HTTPAPIConsole, error) {
	url, err := url.Parse(restAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse restAddr: %w", err)
	}

	return restclient.NewHTTPClientWithConfig(nil, &restclient.TransportConfig{
		Host:     url.Host,
		BasePath: restClientBasePath,
		Schemes:  restClientSchemes,
	}), nil
}

// getNeutronRegisteredQuery retrieves a registered query from Neutron.
func (s *Subscriber) getNeutronRegisteredQuery(ctx context.Context, queryId string) (*neutrontypes.RegisteredQuery, error) {
	res, err := s.restClient.Query.NeutronInterchainadapterInterchainqueriesRegisteredQuery(
		&query.NeutronInterchainadapterInterchainqueriesRegisteredQueryParams{
			QueryID: &queryId,
			Context: ctx,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get NeutronInterchainadapterInterchainqueriesRegisteredQueries: %w", err)
	}

	neutronQuery, err := res.GetPayload().RegisteredQuery.ToNeutronRegisteredQuery()
	if err != nil {
		return nil, fmt.Errorf("failed to get neutronQueryFromRestQuery: %w", err)
	}

	return neutronQuery, nil
}

// getActiveQueries retrieves the list of registered queries filtered by owner, connection, and query type.
func (s *Subscriber) getNeutronRegisteredQueries(ctx context.Context) (map[string]*neutrontypes.RegisteredQuery, error) {
	// TODO: use pagination.
	res, err := s.restClient.Query.NeutronInterchainadapterInterchainqueriesRegisteredQueries(
		&query.NeutronInterchainadapterInterchainqueriesRegisteredQueriesParams{
			Owners:       s.registry.GetAddresses(),
			ConnectionID: &s.targetConnectionID,
			Context:      ctx,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get NeutronInterchainadapterInterchainqueriesRegisteredQueries: %w", err)
	}

	var (
		payload = res.GetPayload()
		out     = map[string]*neutrontypes.RegisteredQuery{}
	)

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

	return out, nil
}

// checkEvents verifies that 1. there is N events associated with the connection id that we are
// interested in, 2. there is a matching number of other query-specific event attributes.
func (s *Subscriber) checkEvents(event tmtypes.ResultEvent) (bool, error) {
	events := event.Events

	icqEventsCount := len(events[connectionIdAttr])
	if icqEventsCount == 0 {
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

// subscribeQuery returns the subscriber name.
// Note: it doesn't matter what we return here because Tendermint will override it with
// remote IP anyway.
func (s *Subscriber) subscriberName() string {
	return s.targetChainID + "-rpcClient"
}

// subscribeQuery returns a ActiveQuery to filter out interchain ActiveQuery events.
func (s *Subscriber) subscribeQueryUpdated() string {
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s'",
		connectionIdAttr, s.targetConnectionID,
		moduleAttr, neutrontypes.ModuleName,
		actionAttr, neutrontypes.AttributeValueQueryUpdated,
	)
}

// subscribeQuery returns a ActiveQuery to filter out interchain ActiveQuery events.
func (s *Subscriber) subscribeQueryRemoved() string {
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s'",
		connectionIdAttr, s.targetConnectionID,
		moduleAttr, neutrontypes.ModuleName,
		actionAttr, neutrontypes.AttributeValueQueryRemoved,
	)
}

// subscribeQuery returns a ActiveQuery to filter out interchain ActiveQuery events.
func (s *Subscriber) subscribeQueryBlock() string {
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
