package subscriber

import (
	"fmt"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func (s *Subscriber) getNeutronRegisteredQuery(queryId string) (*neutrontypes.RegisteredQuery, error) {
	res, err := s.restClient.Query.NeutronInterchainadapterInterchainqueriesRegisteredQuery(
		&query.NeutronInterchainadapterInterchainqueriesRegisteredQueryParams{
			QueryID: &queryId,
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

// getActiveQueries TODO(oopcode).
func (s *Subscriber) getNeutronRegisteredQueries() (map[string]*neutrontypes.RegisteredQuery, error) {
	res, err := s.restClient.Query.NeutronInterchainadapterInterchainqueriesRegisteredQueries(
		&query.NeutronInterchainadapterInterchainqueriesRegisteredQueriesParams{
			Owners:       s.registry.GetAddresses(),
			ConnectionID: &s.targetConnectionID,
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
			return nil, fmt.Errorf("failed to get ToNeutronRegisteredQuery: %w", err)
		}

		out[restQuery.ID] = neutronQuery
	}

	return out, nil
}

// checkEvents TODO(oopcode).
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
	return fmt.Sprintf("%s='%s' AND %s='%s' AND %s='%s' AND %s='%s'",
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
func (s *Subscriber) isWatchedMsgType(msgType neutrontypes.InterchainQueryType) bool {
	_, ex := s.watchedTypes[msgType]
	return ex
}

// isWatchedAddress returns true if the address is within the registry watched addresses or there
// are no registry watched addresses configured for the subscriber meaning all addresses are watched.
func (s *Subscriber) isWatchedAddress(address string) bool {
	return s.registry.IsEmpty() || s.registry.Contains(address)
}

// extractBlockHeight TODO(oopcode).
func (s *Subscriber) extractBlockHeight(event tmtypes.ResultEvent) (uint64, error) {
	// TODO(oopcode): implement
	return 0, nil
}
