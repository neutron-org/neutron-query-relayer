package relay

import (
	"context"
	"fmt"
	"github.com/avast/retry-go/v4"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	relayermetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	"go.uber.org/zap"
	"time"
)

// How many consensusStates to retrieve for each page in `qc.ConsensusStates(...)` call
const consensusPageSize = 10

// submissionMarginPeriod is a lag period, because we need consensusState to be valid until we approve it on the chain
const submissionMarginPeriod time.Duration = time.Minute * 5

var (
	RtyAttNum = uint(5)
	RtyAtt    = retry.Attempts(RtyAttNum)
	RtyDel    = retry.Delay(time.Millisecond * 400)
	RtyErr    = retry.LastErrorOnly(true)
)

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
// `suitableConsensusState` - any consensus state that has height < supplied height
// `height` - height for a header we'll get
func (cm *ConsensusManager) getHeaderWithTrustedHeight(ctx context.Context, suitableConsensusState *clienttypes.ConsensusStateWithHeight, height uint64) (ibcexported.Header, error) {
	start := time.Now()

	cm.logger.Info("Found suitable consensus state with trusted height", zap.Uint64("height", suitableConsensusState.Height.RevisionHeight))

	header, err := cm.targetChain.ChainProvider.GetLightSignedHeaderAtHeight(ctx, int64(height))
	if err != nil {
		relayermetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("failed to get light signed header: %w", err)
	}

	relayermetrics.AddSuccessTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())

	resultHeader, err := InjectTrustedFields(ctx, cm.targetChain.ChainProvider, header, suitableConsensusState)
	if err != nil {
		return nil, fmt.Errorf("failed to inject trusted fields into tmHeader: %w", err)
	}

	return resultHeader, err
}

// getConsensusStateWithTrustedHeight tries to find any consensusState within trusted period with a height <= supplied height
// To do this, it simply iterates over all consensus states and checks each that it's within trusted period AND <= supplied_height
// Note that we cannot optimize this search due to consensus states being stored in a tree with *STRING* key `RevisionNumber-RevisionHeight`
//
// Arguments:
// `height` - found consensus state will be with a height <= than it
func (cm *ConsensusManager) getConsensusStateWithTrustedHeight(ctx context.Context, height uint64) (*clienttypes.ConsensusStateWithHeight, error) {
	// Without this hack it doesn't want to work with NewQueryClient
	neutronProvider, ok := cm.neutronChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}

	qc := clienttypes.NewQueryClient(neutronProvider)

	trustingPeriod, err := cm.fetchTrustingPeriod(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch trusting period: %w", err)
	}

	nextKey := make([]byte, 0)

	for {
		page, err := qc.ConsensusStates(ctx, &clienttypes.QueryConsensusStatesRequest{
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

		for _, item := range page.ConsensusStates {
			unpackedItem, ok := item.ConsensusState.GetCachedValue().(ibcexported.ConsensusState)
			if !ok {
				return nil, fmt.Errorf("could not unpack consensus state from Any value")
			}

			consensusTimestamp := time.Unix(0, int64(unpackedItem.GetTimestamp()))
			now := time.Now()

			olderThanCurrentHeight := height >= item.Height.RevisionHeight
			notExpired := consensusTimestamp.Add(trustingPeriod).Add(-submissionMarginPeriod).After(now)

			if olderThanCurrentHeight && notExpired {
				cm.logger.Debug("Found suitable consensus state",
					zap.Uint64("height", item.Height.RevisionHeight))
				return &item, nil
			}
		}

		nextKey = page.GetPagination().NextKey

		if len(nextKey) == 0 {
			break
		}
	}

	return nil, fmt.Errorf("could not find any trusted consensus state for height=%d", height)
}

func (cm *ConsensusManager) fetchTrustingPeriod(ctx context.Context) (time.Duration, error) {
	clientState, err := cm.neutronChain.ChainProvider.QueryClientState(ctx, 0, cm.neutronChain.PathEnd.ClientID)
	if err != nil {
		return 0, fmt.Errorf("could not fetch client state for ClientId=%s: %w", cm.neutronChain.PathEnd.ClientID, err)
	}

	tmClientState := clientState.(*tmclient.ClientState)
	if tmClientState.TrustingPeriod == 0 {
		return 0, fmt.Errorf("got empty TrustingPeriod")
	}

	return tmClientState.TrustingPeriod, nil
}

// InjectTrustedFields injects TrustedHeight and TrustedValidators into header
func InjectTrustedFields(ctx context.Context, targetProvider provider.ChainProvider, header ibcexported.Header, suitableConsensusState *clienttypes.ConsensusStateWithHeight) (ibcexported.Header, error) {
	// make copy of header stored in mop
	tmHeader, ok := header.(*tmclient.Header)
	if !ok {
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
	}

	// inject TrustedHeight
	tmHeader.TrustedHeight = suitableConsensusState.Height

	// NOTE: We need to get validators from the source chain at height: trustedHeight+1
	// since the last trusted validators for a header at height h is the NextValidators
	// at h+1 committed to in header h by NextValidatorsHash

	var trustedHeader *tmclient.Header
	if err := retry.Do(func() error {
		nextHeader, err := targetProvider.GetLightSignedHeaderAtHeight(ctx, int64(tmHeader.TrustedHeight.RevisionHeight+1))
		if err != nil {
			return err
		}

		nextTmHeader, ok := nextHeader.(*tmclient.Header)
		if !ok {
			err = fmt.Errorf("non-tm client header")
		}

		trustedHeader = nextTmHeader
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return nil, fmt.Errorf(
			"failed to get trusted header, please ensure header at the height %d has not been pruned by the connected node: %w",
			tmHeader.TrustedHeight.RevisionHeight, err,
		)
	}

	// inject TrustedValidators into header
	tmHeader.TrustedValidators = trustedHeader.ValidatorSet

	return tmHeader, nil
}
