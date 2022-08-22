package relay

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/types/query"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	relayermetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	"time"
)

// How many consensusStates to retrieve for each page in `qc.ConsensusStates(...)` call
const consensusPageSize = 100

// Questions:
// - can we reuse consensus states?
// - are they immutable OR are they updated somehow?
// - if they are updated, how do we recache them?
// - or just leave as it was

// ConsensusManager manages the consensus state
// TODO: only GetHeaderWithBestTrustedHeight should be public
// GetConsensusStates results should probably be cached
type ConsensusManager struct {
	neutronChain *relayer.Chain
	targetChain  *relayer.Chain
}

func NewConsensusManager(neutronChain *relayer.Chain, targetChain *relayer.Chain) ConsensusManager {
	return ConsensusManager{
		neutronChain: neutronChain,
		targetChain:  targetChain,
	}
}

// GetConsensusStates returns light client consensus states from Neutron chain
func (cm *ConsensusManager) GetConsensusStates(ctx context.Context) ([]clienttypes.ConsensusStateWithHeight, error) {
	// Without this hack it doesn't want to work with NewQueryClient
	provConcreteNeutronChain, ok := cm.neutronChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}

	qc := clienttypes.NewQueryClient(provConcreteNeutronChain)

	nextKey := make([]byte, 0)
	var res []clienttypes.ConsensusStateWithHeight

	for {
		consensusStatesResponse, err := qc.ConsensusStates(ctx, &clienttypes.QueryConsensusStatesRequest{
			ClientId: cm.neutronChain.ClientID(),
			Pagination: &query.PageRequest{
				Key:        nextKey,
				Limit:      consensusPageSize,
				Reverse:    true,
				CountTotal: true,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get consensus states for client ID %s: %w", cm.neutronChain.ClientID(), err)
		}

		res = append(res, consensusStatesResponse.ConsensusStates...)

		nextKey = consensusStatesResponse.GetPagination().NextKey

		// no more elements left to query
		// TODO: check if this is working and correct
		if nextKey == nil || len(nextKey) <= 0 {
			break
		}
	}

	return res, nil
}

// GetHeaderWithBestTrustedHeight returns an IBC Update Header which can be used to update an on chain
// light client on the Neutron chain.
//
// It has the same purpose as r.targetChain.ChainProvider.GetIBCUpdateHeader() but the difference is
// that getHeaderWithBestTrustedHeight() trys to find the best TrustedHeight for the header
// relying on existing light client's consensus states on the Neutron chain.
//
// The best trusted height for the height in this case is the closest one to some existed consensus state's height but not less
// TODO: test
func (cm *ConsensusManager) GetHeaderWithBestTrustedHeight(ctx context.Context, consensusStates []clienttypes.ConsensusStateWithHeight, height uint64) (ibcexported.Header, error) {
	start := time.Now()
	bestTrustedHeight := clienttypes.Height{
		RevisionNumber: 0,
		RevisionHeight: 0,
	}

	// TODO: since we should implement paging for getting the consensus states, maybe it's better to move searching of
	// 	the best height there
	for _, cs := range consensusStates {
		if height >= cs.Height.RevisionHeight && cs.Height.RevisionHeight > bestTrustedHeight.RevisionHeight {
			bestTrustedHeight = cs.Height
			// we won't find anything better
			if cs.Height.RevisionHeight == height {
				break
			}
		}
	}

	if bestTrustedHeight.IsZero() {
		return nil, fmt.Errorf("no satisfying trusted height found for height: %v", height)
	}

	// Without this hack we can't call InjectTrustedFields
	provConcreteTargetChain, ok := cm.targetChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		relayermetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}
	header, err := cm.targetChain.ChainProvider.GetLightSignedHeaderAtHeight(ctx, int64(height))
	if err != nil {
		relayermetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, err
	}

	tmHeader, ok := header.(*tmclient.Header)
	if !ok {
		relayermetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
	}

	tmHeader.TrustedHeight = bestTrustedHeight
	relayermetrics.AddSuccessTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
	return provConcreteTargetChain.InjectTrustedFields(ctx, tmHeader, cm.neutronChain.ChainProvider, cm.neutronChain.PathEnd.ClientID)
}
