package relay

import (
	"context"
	"fmt"
	"time"

	neutronmetrics "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"

	"github.com/avast/retry-go/v4"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"
)

// consensusPageSize is how many consensusStates to retrieve for each page in `qc.ConsensusStates(...)` call
const consensusPageSize = 10

// submissionMarginPeriod is a lag period, because we need consensusState to be valid until we approve it on the chain
const submissionMarginPeriod = time.Minute * 5

// retries configuration for fetching light header
var (
	RtyAttNum = uint(5)
	RtyAtt    = retry.Attempts(RtyAttNum)
	RtyDel    = retry.Delay(time.Millisecond * 400)
	RtyErr    = retry.LastErrorOnly(true)
)

// TrustedHeaderFetcher able to get trusted headers for a given height
// Trusted headers are needed in Neutron along with proofs to verify that transactions are:
// - included in the block (inclusion proof)
// - successfully executed (delivery proof)
type TrustedHeaderFetcher struct {
	neutronChain   *relayer.Chain
	targetChain    *relayer.Chain
	logger         *zap.Logger
	revisionNumber uint64
}

// NewTrustedHeaderFetcher constructs a new TrustedHeaderFetcher
func NewTrustedHeaderFetcher(neutronChain *relayer.Chain, targetChain *relayer.Chain, logger *zap.Logger) *TrustedHeaderFetcher {
	return &TrustedHeaderFetcher{
		neutronChain:   neutronChain,
		targetChain:    targetChain,
		logger:         logger,
		revisionNumber: clienttypes.ParseChainID(targetChain.ChainID()),
	}
}

// Fetch returns two Headers for height and height+1 packed into *codectypes.Any value
// We need two blocks in Neutron to verify both delivery of tx and inclusion in block:
// - We need to know block X (`header`) to verify inclusion of transaction for block X (inclusion proof)
// - We need to know block X+1 (`nextHeader`) to verify response of transaction for block X
// since LastResultsHash is root hash of all results from the txs from the previous block (delivery proof)
//
// Arguments:
// `height` - remote chain block height X = transaction with such block height
func (thf *TrustedHeaderFetcher) Fetch(ctx context.Context, height uint64) (header *codectypes.Any, nextHeader *codectypes.Any, err error) {
	start := time.Now()

	// tries to find height of the closest consensus state height that is less or equal than provided height
	trustedHeight, err := thf.getTrustedHeight(ctx, height)
	if err != nil {
		err = fmt.Errorf("no satisfying consensus state found: %w", err)
		return
	}
	thf.logger.Debug("Found suitable consensus state with trusted height", zap.Uint64("height", trustedHeight.RevisionHeight))

	header, err = thf.packedTrustedHeaderAtHeight(ctx, trustedHeight, height)
	if err != nil {
		err = fmt.Errorf("failed to get header for src chain: %w", err)
		return
	}

	nextHeader, err = thf.packedTrustedHeaderAtHeight(ctx, trustedHeight, height+1)
	if err != nil {
		err = fmt.Errorf("failed to get next header for src chain: %w", err)
		return
	}

	neutronmetrics.RecordActionDuration("TrustedHeaderFetcher", time.Since(start).Seconds())

	return
}

// packedTrustedHeaderAtHeight finds trusted header at height and packs it for sending
func (thf *TrustedHeaderFetcher) packedTrustedHeaderAtHeight(ctx context.Context, trustedHeight *clienttypes.Height, height uint64) (*codectypes.Any, error) {
	header, err := thf.trustedHeaderAtHeight(ctx, trustedHeight, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get header with trusted height: %w", err)
	}

	packedHeader, err := clienttypes.PackHeader(header)
	if err != nil {
		return nil, fmt.Errorf("failed to pack header: %w", err)
	}

	return packedHeader, nil
}

// trustedHeaderAtHeight returns a Header with injected necessary trusted fields (TrustedHeight and TrustedValidators) for a height
// to update a light client stored on the Neutron, using the information provided by the target chain.
//
// TrustedHeight is a height of the IBC client on Neutron for the provided height to trust
// TrustedValidators is the validator set of target chain at the TrustedHeight + 1.
//
// The function is very similar to InjectTrustedFields (https://github.com/cosmos/relayer/blob/v2.0.0-beta7/relayer/provider/cosmos/provider.go#L727)
// but with one big difference: trustedHeaderAtHeight injects trusted fields for a particular trusted height, not the latest one in IBC light client.
// This allows us to send UpdateClient msg not only for new heights, but for the old ones (which are still in the trusting period).
//
// Arguments:
// `trustedHeight` - height of any consensus state that's height <= supplied height
// `height` - remote chain height for a header
func (thf *TrustedHeaderFetcher) trustedHeaderAtHeight(ctx context.Context, trustedHeight *clienttypes.Height, height uint64) (ibcexported.Header, error) {
	header, err := thf.retryGetLightSignedHeaderAtHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not get light header: %w", err)
	}

	// NOTE: We need to get validators from the source chain at height: trustedHeight+1
	// since the last trusted validators for a header at height h is the NextValidators
	// at h+1 committed to in header h by NextValidatorsHash
	nextHeader, err := thf.retryGetLightSignedHeaderAtHeight(ctx, trustedHeight.RevisionHeight+1)
	if err != nil {
		return nil, fmt.Errorf("could not get next light header: %w", err)
	}

	// inject TrustedHeight and TrustedValidators into header
	header.TrustedHeight = *trustedHeight
	header.TrustedValidators = nextHeader.ValidatorSet

	return header, nil
}

// getTrustedHeight tries to find height of any consensusState within trusting period with a height <= supplied height
// To do this, it simply iterates over all consensus states and checks each that it's within trusting period AND <= supplied_height
// Note that we cannot optimize this search due to consensus states being stored in a tree with *STRING* key `RevisionNumber-RevisionHeight`
//
// Arguments:
// `height` - found consensus state will be with a height <= than it
func (thf *TrustedHeaderFetcher) getTrustedHeight(ctx context.Context, height uint64) (*clienttypes.Height, error) {
	// Without this hack it doesn't want to work with NewQueryClient
	neutronProvider, ok := thf.neutronChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}

	qc := clienttypes.NewQueryClient(neutronProvider)

	trustingPeriod, err := thf.fetchTrustingPeriod(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch trusting period: %w", err)
	}
	thf.logger.Debug("fetched trusting period", zap.Float64("trusting_period_hours", trustingPeriod.Hours()))

	nextKey := make([]byte, 0)

	for {
		page, err := qc.ConsensusStates(ctx, requestPage(thf.neutronChain.ClientID(), nextKey))
		if err != nil {
			return nil, fmt.Errorf("failed to get consensus states for client ID %s: %w", thf.neutronChain.ClientID(), err)
		}

		for _, cs := range page.ConsensusStates {
			suitable, err := thf.isSuitableCS(cs, trustingPeriod, height)
			if err != nil {
				return nil, err
			} else if suitable {
				return &cs.Height, nil
			}
		}

		nextKey = page.GetPagination().NextKey

		if len(nextKey) == 0 {
			break
		}
	}

	return nil, fmt.Errorf("could not find any trusted consensus state for height=%d", height)
}

// fetchTrustingPeriod fetches trusting period of the client
func (thf *TrustedHeaderFetcher) fetchTrustingPeriod(ctx context.Context) (time.Duration, error) {
	clientState, err := thf.neutronChain.ChainProvider.QueryClientState(ctx, 0, thf.neutronChain.PathEnd.ClientID)
	if err != nil {
		return 0, fmt.Errorf("could not fetch client state for ClientId=%s: %w", thf.neutronChain.PathEnd.ClientID, err)
	}

	tmClientState, ok := clientState.(*tmclient.ClientState)
	if !ok {
		// todo: correct to print nonconverted var?
		return 0, fmt.Errorf("expected client state of type *tmclient.ClientState, got %T", tmClientState)
	}
	if tmClientState.TrustingPeriod == 0 {
		return 0, fmt.Errorf("got empty TrustingPeriod")
	}

	return tmClientState.TrustingPeriod, nil
}

func (thf *TrustedHeaderFetcher) retryGetLightSignedHeaderAtHeight(ctx context.Context, height uint64) (*tmclient.Header, error) {
	var tmHeader *tmclient.Header

	if err := retry.Do(func() error {
		header, err := thf.targetChain.ChainProvider.GetLightSignedHeaderAtHeight(ctx, int64(height))
		if err != nil {
			return err
		}

		tmp, ok := header.(*tmclient.Header)
		if !ok {
			return fmt.Errorf("expected header of type *tmclient.Header, got %T", header)
		}

		tmHeader = tmp
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return nil, fmt.Errorf(
			"failed to get trusted header, please ensure header at the height %d has not been pruned by the connected node: %w",
			tmHeader.TrustedHeight.RevisionHeight, err,
		)
	}

	return tmHeader, nil
}

// isSuitableCS parses the given consensus state and checks if its height can be used as trusted.
// The condition for this check is the following:
//
// 1. The consensus state height is in the same revision and is less or equal to the given height;
// 2. The consensus state timestamp is within the trusting period.
func (thf *TrustedHeaderFetcher) isSuitableCS(cs clienttypes.ConsensusStateWithHeight, trustingPeriod time.Duration, height uint64) (bool, error) {
	ibcCS, ok := cs.ConsensusState.GetCachedValue().(ibcexported.ConsensusState)
	if !ok {
		return false, fmt.Errorf("couldn't cast consensus state value of type %T to ibcexported.ConsensusState", cs.ConsensusState.GetCachedValue())
	}
	olderOrSameAsHeightInSameRevision := cs.Height.RevisionNumber == thf.revisionNumber && cs.Height.RevisionHeight <= height
	consensusTimestamp := time.Unix(0, int64(ibcCS.GetTimestamp()))
	inTrustingPeriod := consensusTimestamp.Add(trustingPeriod).Add(-submissionMarginPeriod).After(time.Now())
	return olderOrSameAsHeightInSameRevision && inTrustingPeriod, nil
}

func requestPage(clientID string, nextKey []byte) *clienttypes.QueryConsensusStatesRequest {
	return &clienttypes.QueryConsensusStatesRequest{
		ClientId: clientID,
		Pagination: &query.PageRequest{
			Key:        nextKey,
			Limit:      consensusPageSize,
			Reverse:    true,
			CountTotal: false,
		},
	}
}
