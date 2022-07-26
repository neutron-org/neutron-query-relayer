package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/cosmos/cosmos-sdk/types/query"
	neutronmetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"math"
	"strconv"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
)

// Relayer is controller for the whole app:
// 1. takes events from Neutron chain
// 2. dispatches each query by type to fetch proof for the right query
// 3. submits proof for a query back to the Neutron chain
type Relayer struct {
	proofer      Proofer
	submitter    Submitter
	targetChain  *relayer.Chain
	neutronChain *relayer.Chain
	logger       *zap.Logger

	targetChainId     string
	targetChainPrefix string
}

func NewRelayer(
	proofer Proofer,
	submitter Submitter,
	targetChainId,
	targetChainPrefix string,
	srcChain,
	dstChain *relayer.Chain,
	logger *zap.Logger,
) Relayer {
	return Relayer{
		proofer:           proofer,
		submitter:         submitter,
		targetChainId:     targetChainId,
		targetChainPrefix: targetChainPrefix,
		targetChain:       srcChain,
		neutronChain:      dstChain,
		logger:            logger,
	}
}

func (r Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) error {
	messages, err := r.tryExtractInterchainQueries(event)
	if err != nil {
		return fmt.Errorf("could not filter interchain query messages: %w", err)
	}

	for _, m := range messages {
		start := time.Now()
		err := r.proofMessage(ctx, m)
		if err != nil {
			r.logger.Error("could not process message", zap.Uint64("query_id", m.queryId), zap.Error(err))
			neutronmetrics.IncFailedProofs()
			neutronmetrics.AddFailedRequest(string(m.messageType), time.Since(start).Seconds())
		} else {
			neutronmetrics.IncSuccessProofs()
			neutronmetrics.AddSuccessRequest(string(m.messageType), time.Since(start).Seconds())
			r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", m.queryId))
		}
	}

	return nil
}

func (r Relayer) tryExtractInterchainQueries(event coretypes.ResultEvent) ([]queryEventMessage, error) {
	r.logger.Info("extracting events", zap.Any("events", event.Events))
	events := event.Events
	if len(events[zoneIdAttr]) == 0 {
		return nil, nil
	}

	if len(events[zoneIdAttr]) != len(events[kvKeyAttr]) ||
		len(events[zoneIdAttr]) != len(events[transactionsFilter]) ||
		len(events[zoneIdAttr]) != len(events[queryIdAttr]) ||
		len(events[zoneIdAttr]) != len(events[typeAttr]) {
		return nil, fmt.Errorf("cannot filter interchain query messages because events attributes length does not match for events=%v", events)
	}

	messages := make([]queryEventMessage, 0, len(events[zoneIdAttr]))

	for idx, zoneId := range events[zoneIdAttr] {
		if zoneId != r.targetChainId {
			continue
		}

		queryIdStr := events[queryIdAttr][idx]
		queryId, err := strconv.ParseUint(queryIdStr, 10, 64)
		if err != nil {
			r.logger.Info("invalid query_id format (not an uint)", zap.Error(err))
			continue
		}

		var (
			kvKeys                  neutrontypes.KVKeys
			transactionsFilterValue string
		)

		messageType := neutrontypes.InterchainQueryType(events[typeAttr][idx])

		switch messageType {
		case neutrontypes.InterchainQueryTypeKV:
			kvKeys, err = neutrontypes.KVKeysFromString(events[kvKeyAttr][idx])
			if err != nil {
				fmt.Printf("invalid kv_key attr: %v", err)
				continue
			}
		case neutrontypes.InterchainQueryTypeTX:
			transactionsFilterValue = events[transactionsFilter][idx]
		default:
			fmt.Printf("unknown query_type: %s", messageType)
			continue
		}

		messages = append(messages, queryEventMessage{queryId: queryId, messageType: messageType, kvKeys: kvKeys, transactionsFilter: transactionsFilterValue})
	}

	return messages, nil
}

func (r Relayer) proofMessage(ctx context.Context, m queryEventMessage) error {

	r.logger.Info("proofMessage message_type", zap.String("message_type", string(m.messageType)))

	latestHeight, err := r.targetChain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to QueryLatestHeight: %w", err)
	}

	updateClientMsg, err := r.getUpdateClientMsg(ctx, latestHeight)
	if err != nil {
		return fmt.Errorf("failed to getUpdateClientMsg: %w", err)
	}

	srcHeader, err := r.getSrcChainHeader(ctx, latestHeight)
	if err != nil {
		return fmt.Errorf("failed to get header for height: %d: %w", latestHeight, err)
	}

	switch m.messageType {
	case neutrontypes.InterchainQueryTypeKV:
		proofs, height, err := r.proofer.GetStorageValuesWithProof(ctx, uint64(latestHeight), m.kvKeys)
		proofStart := time.Now()
		err = r.submitter.SubmitProof(ctx, height, srcHeader.GetHeight().GetRevisionNumber(), m.queryId, proofs, updateClientMsg)
		if err != nil {
			neutronmetrics.AddFailedProof(string(m.messageType), time.Since(proofStart).Seconds())
			return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
		}

		neutronmetrics.AddSuccessProof(string(m.messageType), time.Since(proofStart).Seconds())
	case neutrontypes.InterchainQueryTypeTX:
		var params recipientTransactionsParams
		err := json.Unmarshal([]byte(m.transactionsFilter), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal transactions filter for %s with params=%s query_id=%d: %w",
				m.messageType, m.transactionsFilter, m.queryId, err)
		}

		blocks, err := r.proofer.SearchTransactionsWithProofs(ctx, params)
		if err != nil {
			return fmt.Errorf("could not get proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
		}

		consensusStates, err := r.getConsensusStates(ctx)
		if err != nil {
			return fmt.Errorf("failed to get consensus states: %w", err)
		}

		resultBlocks := make([]*neutrontypes.Block, 0, len(blocks))
		for height, txs := range blocks {
			header, err := r.getHeaderWithBestTrustedHeight(ctx, consensusStates, height)
			if err != nil {
				return fmt.Errorf("failed to get header for src chain: %w", err)
			}

			packedHeader, err := clienttypes.PackHeader(header)
			if err != nil {
				return fmt.Errorf("failed to pack header: %w", err)
			}

			nextHeader, err := r.getHeaderWithBestTrustedHeight(ctx, consensusStates, height+1)
			if err != nil {
				return fmt.Errorf("failed to get next header for src chain: %w", err)
			}

			packedNextHeader, err := clienttypes.PackHeader(nextHeader)
			if err != nil {
				return fmt.Errorf("failed to pack header: %w", err)
			}

			resultBlocks = append(resultBlocks, &neutrontypes.Block{
				Header:          packedHeader,
				NextBlockHeader: packedNextHeader,
				Txs:             txs,
			})
		}

		proofStart := time.Now()
		err = r.submitter.SubmitTxProof(ctx, m.queryId, r.neutronChain.PathEnd.ClientID, resultBlocks)
		if err != nil {
			neutronmetrics.AddFailedProof(string(m.messageType), time.Since(proofStart).Seconds())
			return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
		}
		neutronmetrics.AddSuccessProof(string(m.messageType), time.Since(proofStart).Seconds())

	default:
		return fmt.Errorf("unknown query messageType=%s", m.messageType)
	}

	return nil
}

// getConsensusStates returns light client consensus states from Neutron chain
func (r *Relayer) getConsensusStates(ctx context.Context) ([]clienttypes.ConsensusStateWithHeight, error) {
	// Without this hack it doesn't want to work with NewQueryClient
	provConcreteNeutronChain, ok := r.neutronChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}

	qc := clienttypes.NewQueryClient(provConcreteNeutronChain)

	consensusStatesResponse, err := qc.ConsensusStates(ctx, &clienttypes.QueryConsensusStatesRequest{
		ClientId: r.neutronChain.ClientID(),
		Pagination: &query.PageRequest{
			// TODO: paging
			Limit:      math.MaxUint64,
			Reverse:    true,
			CountTotal: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get consensus states for client ID %s: %w", r.neutronChain.ClientID(), err)
	}

	return consensusStatesResponse.ConsensusStates, nil
}

// getHeaderWithBestTrustedHeight returns an IBC Update Header which can be used to update an on chain
// light client on the Neutron chain.
//
// It has the same purpose as r.targetChain.ChainProvider.GetIBCUpdateHeader() but the difference is
// that getHeaderWithBestTrustedHeight() trys to find the best TrustedHeight for the header
// relying on existing light client's consensus states on the Neutron chain.
//
// The best trusted height for the height in this case is the closest one to some existed consensus state's height but not less
func (r *Relayer) getHeaderWithBestTrustedHeight(ctx context.Context, consensusStates []clienttypes.ConsensusStateWithHeight, height uint64) (ibcexported.Header, error) {
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
	provConcreteTargetChain, ok := r.targetChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		neutronmetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}
	header, err := r.targetChain.ChainProvider.GetLightSignedHeaderAtHeight(ctx, int64(height))
	if err != nil {
		neutronmetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, err
	}

	tmHeader, ok := header.(*tmclient.Header)
	if !ok {
		neutronmetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
	}

	tmHeader.TrustedHeight = bestTrustedHeight
	neutronmetrics.AddSuccessTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
	return provConcreteTargetChain.InjectTrustedFields(ctx, tmHeader, r.neutronChain.ChainProvider, r.neutronChain.PathEnd.ClientID)
}

func (r *Relayer) getSrcChainHeader(ctx context.Context, height int64) (ibcexported.Header, error) {
	start := time.Now()
	var srcHeader ibcexported.Header
	if err := retry.Do(func() error {
		var err error
		srcHeader, err = r.targetChain.ChainProvider.GetIBCUpdateHeader(ctx, height, r.neutronChain.ChainProvider, r.neutronChain.PathEnd.ClientID)
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		r.logger.Info(
			"failed to GetIBCUpdateHeader", zap.Error(err))
	})); err != nil {
		neutronmetrics.AddFailedTargetChainGetter("GetIBCUpdateHeader", time.Since(start).Seconds())
		return nil, err
	}
	neutronmetrics.AddSuccessTargetChainGetter("GetIBCUpdateHeader", time.Since(start).Seconds())
	return srcHeader, nil
}

func (r *Relayer) getUpdateClientMsg(ctx context.Context, targeth int64) (sdk.Msg, error) {
	start := time.Now()
	// Query IBC Update Header
	srcHeader, err := r.getSrcChainHeader(ctx, targeth)
	if err != nil {
		neutronmetrics.AddFailedTargetChainGetter("GetUpdateClientMsg", time.Since(start).Seconds())
		return nil, err
	}

	// Construct UpdateClient msg
	var updateMsgRelayer provider.RelayerMessage
	if err := retry.Do(func() error {
		var err error
		updateMsgRelayer, err = r.neutronChain.ChainProvider.UpdateClient(r.neutronChain.PathEnd.ClientID, srcHeader)
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		r.logger.Error(
			"failed to build message", zap.Error(err))
	})); err != nil {
		return nil, err
	}

	updateMsgUnpacked, ok := updateMsgRelayer.(cosmos.CosmosMessage)
	if !ok {
		return nil, errors.New("failed to cast provider.RelayerMessage to cosmos.CosmosMessage")
	}
	neutronmetrics.AddSuccessTargetChainGetter("GetUpdateClientMsg", time.Since(start).Seconds())
	return updateMsgUnpacked.Msg, nil
}
