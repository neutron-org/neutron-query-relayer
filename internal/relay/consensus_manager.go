package relay

import (
	"context"
	"fmt"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	relayermetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	"go.uber.org/zap"
	"strings"
	"time"
)

// How many consensusStates to retrieve for each page in `qc.ConsensusStates(...)` call
const consensusPageSize = 10

// ConsensusManager manages the consensus state
type ConsensusManager struct {
	neutronChain *relayer.Chain
	targetChain  *relayer.Chain
	logger       *zap.Logger
}

func NewConsensusManager(neutronChain *relayer.Chain, targetChain *relayer.Chain, logger *zap.Logger) ConsensusManager {
	return ConsensusManager{
		neutronChain: neutronChain,
		targetChain:  targetChain,
		logger:       logger,
	}
}

// GetPackedHeadersWithTrustedHeight returns two IBC Update headers for height and height+1 packed into *codectypes.Any value
//
// Arguments:
// `ctx` - context
// `height` - we use this height to get headers for height and height+1
func (cm *ConsensusManager) GetPackedHeadersWithTrustedHeight(ctx context.Context, height uint64) (header *codectypes.Any, nextHeader *codectypes.Any, err error) {
	// tries to find the closest consensus state height that is less or equal than provided height
	suitableConsensusState, err := cm.getConsensusStateWithTrustedHeight(ctx, height)
	if err != nil {
		err = fmt.Errorf("no satisfying consensus state found: %w", err)
		return
	}

	header, err = cm.getPackedHeaderWithTrustedHeight(ctx, suitableConsensusState, height)
	if err != nil {
		err = fmt.Errorf("failed to get header for src chain: %w", err)
		return
	}

	nextHeader, err = cm.getPackedHeaderWithTrustedHeight(ctx, suitableConsensusState, height+1)
	if err != nil {
		err = fmt.Errorf("failed to get next header for src chain: %w", err)
		return
	}

	return
}

func (cm *ConsensusManager) getPackedHeaderWithTrustedHeight(ctx context.Context, suitableConsensusState *clienttypes.ConsensusStateWithHeight, height uint64) (*codectypes.Any, error) {
	header, err := cm.getHeaderWithTrustedHeight(ctx, suitableConsensusState, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get header with trusted height: %w", err)
	}

	packedHeader, err := clienttypes.PackHeader(header)
	if err != nil {
		return nil, fmt.Errorf("failed to pack header: %w", err)
	}

	return packedHeader, nil
}

// getHeaderWithTrustedHeight returns an IBC Update Header which can be used to update an on chain
// light client on the Neutron chain.
//
// It has the same purpose as r.targetChain.ChainProvider.GetIBCUpdateHeader() but the difference is
// that getHeaderWithTrustedHeight() tries to find the best TrustedHeight for the header
// relying on existing light client's consensus states on the Neutron chain.
//
// Arguments:
// `ctx` - context
// `suitableConsensusState` - any consensus state that has height < supplied height
// `height` - height for a header we'll get
func (cm *ConsensusManager) getHeaderWithTrustedHeight(ctx context.Context, suitableConsensusState *clienttypes.ConsensusStateWithHeight, height uint64) (ibcexported.Header, error) {
	start := time.Now()

	cm.logger.Info("Found suitable consensus state with trusted height", zap.Uint64("height", suitableConsensusState.Height.RevisionHeight))

	// Without this hack we can't call InjectTrustedFields
	provConcreteTargetChain, ok := cm.targetChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		relayermetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}
	header, err := cm.targetChain.ChainProvider.GetLightSignedHeaderAtHeight(ctx, int64(height))
	if err != nil {
		relayermetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("failed to get light signed header: %w", err)
	}

	tmHeader, ok := header.(*tmclient.Header)
	if !ok {
		relayermetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
	}

	tmHeader.TrustedHeight = suitableConsensusState.Height
	relayermetrics.AddSuccessTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
	resultHeader, err := provConcreteTargetChain.InjectTrustedFields(ctx, tmHeader, cm.neutronChain.ChainProvider, cm.neutronChain.PathEnd.ClientID)
	if err != nil {
		return nil, fmt.Errorf("failed to inject trusted fields into tmHeader: %w", err)
	}

	return resultHeader, err
}

// getConsensusStateWithTrustedHeight tries to find any consensusState within trusted period with a height <= supplied height
// returned consensus state will be trusted since ibc-go prunes all expired consensus states (that are not within trusted period)
// To do this, it simply iterates over all consensus states.
// Note that we cannot optimize this due to consensus states being stored in a tree with *STRING* key `RevisionNumber-RevisionHeight`
//
// Arguments:
// `ctx` - context
// `height` - found consensus state will be with a height <= than it
func (cm *ConsensusManager) getConsensusStateWithTrustedHeight(ctx context.Context, height uint64) (*clienttypes.ConsensusStateWithHeight, error) {
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
				CountTotal: false,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get consensus states for client ID %s: %w", cm.neutronChain.ClientID(), err)
		}

		// === TO DEBUG ===
		debugConsensusStateHeights := make([]string, 0)
		for _, c := range consensusStatesResponse.GetConsensusStates() {
			debugConsensusStateHeights = append(debugConsensusStateHeights, fmt.Sprintf("%d-%d", c.Height.RevisionNumber, c.Height.RevisionHeight))
		}
		cm.logger.Info("Consensus states page",
			zap.Int("asked_page_size", consensusPageSize),
			zap.Int("real_page_size", len(debugConsensusStateHeights)),
			zap.Uint64("total", consensusStatesResponse.Pagination.Total),
			zap.String("state_heights", strings.Join(debugConsensusStateHeights, ",")))
		// ================

		res = append(res, consensusStatesResponse.ConsensusStates...)
		for _, item := range consensusStatesResponse.ConsensusStates {
			if height >= item.Height.RevisionHeight {
				cm.logger.Info("Found suitable consensus state",
					zap.Uint64("height", item.Height.RevisionHeight))
				return &item, nil
			}
		}

		nextKey = consensusStatesResponse.GetPagination().NextKey
		//cm.logger.Info("next_key", zap.String("key", fmt.Sprintf("%v", nextKey)))

		if len(nextKey) == 0 {
			cm.logger.Info("!!! key end")
			break
		}
	}

	return nil, fmt.Errorf("could not find any trusted consensus state for height=%d", height)
}
