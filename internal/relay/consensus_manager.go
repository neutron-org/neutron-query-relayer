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
	"github.com/emirpasic/gods/maps/treemap"
	"github.com/emirpasic/gods/utils"
	relayermetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	"go.uber.org/zap"
	"strings"
	"time"
)

// How many consensusStates to retrieve for each page in `qc.ConsensusStates(...)` call
const consensusPageSize = 5

// ConsensusManager manages the consensus state
// TODO: only GetHeaderWithBestTrustedHeight should be public
type ConsensusManager struct {
	consensusStates *treemap.Map
	neutronChain    *relayer.Chain
	targetChain     *relayer.Chain
	logger          *zap.Logger
}

func NewConsensusManager(neutronChain *relayer.Chain, targetChain *relayer.Chain, logger *zap.Logger) ConsensusManager {
	return ConsensusManager{
		consensusStates: treemap.NewWith(utils.UInt64Comparator),
		neutronChain:    neutronChain,
		targetChain:     targetChain,
		logger:          logger,
	}
}

func (cm *ConsensusManager) UpdateConsensusStates(ctx context.Context) error {
	// Without this hack it doesn't want to work with NewQueryClient
	provConcreteNeutronChain, ok := cm.neutronChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		return fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}

	qc := clienttypes.NewQueryClient(provConcreteNeutronChain)

	nextKey := make([]byte, 0)
	//nextOffset := uint64(0)
	var res []clienttypes.ConsensusStateWithHeight

	for {
		consensusStatesResponse, err := qc.ConsensusStates(ctx, &clienttypes.QueryConsensusStatesRequest{
			ClientId: cm.neutronChain.ClientID(),
			Pagination: &query.PageRequest{
				Key: nextKey,
				//Offset:     nextOffset,
				Limit:      consensusPageSize,
				Reverse:    true,
				CountTotal: false,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to get consensus states for client ID %s: %w", cm.neutronChain.ClientID(), err)
		}

		if len(consensusStatesResponse.ConsensusStates) == 0 {
			cm.logger.Info("!!!!!!!!!! key end")
			break
		}

		// === TO DEBUG ===
		stCs := make([]string, 0)
		for _, c := range consensusStatesResponse.GetConsensusStates() {
			stCs = append(stCs, fmt.Sprintf("%d-%d", c.Height.RevisionNumber, c.Height.RevisionHeight))
		}
		cm.logger.Info("Consensus states page",
			zap.Int("asked_page_size", consensusPageSize),
			zap.Int("real_page_size", len(stCs)),
			zap.Uint64("total", consensusStatesResponse.Pagination.Total),
			zap.String("state_heights", strings.Join(stCs, ",")))
		// ================

		res = append(res, consensusStatesResponse.ConsensusStates...)
		for _, cs := range consensusStatesResponse.ConsensusStates {
			cm.consensusStates.Put(cs.Height.RevisionHeight, cs)
		}

		// TODO: if element present in cm.consensusStates, stop processing and return, because they are already in here

		nextKey = consensusStatesResponse.GetPagination().NextKey
		cm.logger.Info("next_key", zap.String("key", fmt.Sprintf("%v", nextKey)))

		//nextOffset = nextOffset + consensusPageSize

		// no more elements left to query
		// TODO: check if this is working and correct
		if nextKey == nil || len(nextKey) == 0 {
			cm.logger.Info("!!!!!!!!!! key end")
			break
		}

		//if (nextOffset > ) {

		//}
	}

	return nil
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

		// === TO DEBUG ===
		stCs := make([]string, 0)
		for _, c := range consensusStatesResponse.GetConsensusStates() {
			stCs = append(stCs, fmt.Sprintf("%d", c.Height.RevisionHeight))
		}
		cm.logger.Info("Consensus states page", zap.Int("page_size", consensusPageSize), zap.String("state_heights", strings.Join(stCs, ",")))
		// ================

		for _, s := range consensusStatesResponse.ConsensusStates {
			cm.logger.Error("failed to loadChains", zap.Uint64("height", s.GetHeight().GetRevisionHeight()))
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

func (cm *ConsensusManager) GetPackedHeaderWithBestTrustedHeight(ctx context.Context, height uint64) (*codectypes.Any, error) {
	header, err := cm.GetHeaderWithBestTrustedHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get header for src chain: %w", err)
	}

	packedHeader, err := clienttypes.PackHeader(header)
	if err != nil {
		return nil, fmt.Errorf("failed to pack header: %w", err)
	}

	return packedHeader, nil
}

// GetHeaderWithBestTrustedHeight returns an IBC Update Header which can be used to update an on chain
// light client on the Neutron chain.
//
// It has the same purpose as r.targetChain.ChainProvider.GetIBCUpdateHeader() but the difference is
// that GetHeaderWithBestTrustedHeight() tries to find the best TrustedHeight for the header
// relying on existing light client's consensus states on the Neutron chain.
func (cm *ConsensusManager) GetHeaderWithBestTrustedHeight(ctx context.Context, height uint64) (ibcexported.Header, error) {
	start := time.Now()
	bestTrustedHeight := clienttypes.Height{
		RevisionNumber: 0,
		RevisionHeight: 0,
	}

	// finds the closest consensus state height that is less or equal than provided height
	_, closestCs := cm.consensusStates.Floor(height)
	if closestCs == nil {
		return nil, fmt.Errorf("no satisfying trusted height found for height: %v", height)
	}

	bestTrustedHeight = closestCs.(clienttypes.ConsensusStateWithHeight).Height

	cm.logger.Debug("Found closest cs", zap.Uint64("height", closestCs.(clienttypes.ConsensusStateWithHeight).Height.RevisionHeight))

	cm.logger.Error("Found best trusted height", zap.Uint64("height", bestTrustedHeight.RevisionHeight))

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
