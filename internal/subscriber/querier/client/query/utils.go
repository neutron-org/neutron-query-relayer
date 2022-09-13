package query

import (
	"fmt"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"strconv"
)

func (o *NeutronInterchainadapterInterchainqueriesRegisteredQueriesOKBodyRegisteredQueriesItems0) ToNeutronRegisteredQuery() (*neutrontypes.RegisteredQuery, error) {
	queryId, err := strconv.ParseUint(o.ID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse o.ID: %w", err)
	}

	updatePeriod, err := strconv.ParseUint(o.UpdatePeriod, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse o.UpdatePeriod: %w", err)
	}

	lastSubmittedResultLocalHeight, err := strconv.ParseUint(o.LastSubmittedResultLocalHeight, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse o.LastSubmittedResultLocalHeight: %w", err)
	}

	lastSubmittedResultRemoteHeight, err := strconv.ParseUint(o.LastSubmittedResultRemoteHeight, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse o.LastSubmittedResultRemoteHeight: %w", err)
	}

	var keys []*neutrontypes.KVKey
	for _, restKey := range o.Keys {
		keys = append(keys, &neutrontypes.KVKey{
			Path: restKey.Path,
			Key:  restKey.Key,
		})
	}

	return &neutrontypes.RegisteredQuery{
		Id:                              queryId,
		Owner:                           o.Owner,
		QueryType:                       o.QueryType,
		Keys:                            keys,
		TransactionsFilter:              o.TransactionsFilter,
		ConnectionId:                    o.ConnectionID,
		UpdatePeriod:                    updatePeriod,
		LastSubmittedResultLocalHeight:  lastSubmittedResultLocalHeight,
		LastSubmittedResultRemoteHeight: lastSubmittedResultRemoteHeight,
	}, nil
}

func (o *NeutronInterchainadapterInterchainqueriesRegisteredQueryOKBodyRegisteredQuery) ToNeutronRegisteredQuery() (*neutrontypes.RegisteredQuery, error) {
	queryId, err := strconv.ParseUint(o.ID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse o.ID: %w", err)
	}

	updatePeriod, err := strconv.ParseUint(o.UpdatePeriod, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse o.UpdatePeriod: %w", err)
	}

	lastSubmittedResultLocalHeight, err := strconv.ParseUint(o.LastSubmittedResultLocalHeight, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse o.LastSubmittedResultLocalHeight: %w", err)
	}

	lastSubmittedResultRemoteHeight, err := strconv.ParseUint(o.LastSubmittedResultRemoteHeight, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse o.LastSubmittedResultRemoteHeight: %w", err)
	}

	var keys []*neutrontypes.KVKey
	for _, restKey := range o.Keys {
		keys = append(keys, &neutrontypes.KVKey{
			Path: restKey.Path,
			Key:  restKey.Key,
		})
	}

	return &neutrontypes.RegisteredQuery{
		Id:                              queryId,
		Owner:                           o.Owner,
		QueryType:                       o.QueryType,
		Keys:                            keys,
		TransactionsFilter:              o.TransactionsFilter,
		ConnectionId:                    o.ConnectionID,
		UpdatePeriod:                    updatePeriod,
		LastSubmittedResultLocalHeight:  lastSubmittedResultLocalHeight,
		LastSubmittedResultRemoteHeight: lastSubmittedResultRemoteHeight,
	}, nil
}
