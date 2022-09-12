package subscriber

import (
	"fmt"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
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
			// TODO(oopcode): add parameters.
			// TODO(oopcode): make Owner repeated?
			// TODO(oopcode): add a connection id filter?
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
