package kvprocessor

import (
	"context"
	"errors"
	"fmt"

	"github.com/avast/retry-go/v4"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron-query-relayer/internal/tmquerier"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// KVProcessor is implementation of relay.KVProcessor that processes event query KV type.
// Obtains the proof for a query we need to process, and sends it to  the neutron
type KVProcessor struct {
	trustedHeaderFetcher relay.TrustedHeaderFetcher
	querier              *tmquerier.Querier
	logger               *zap.Logger
	submitter            relay.Submitter
	storage              relay.Storage
	targetChain          *relayer.Chain
	neutronChain         *relayer.Chain
}

func NewKVProcessor(
	trustedHeaderFetcher relay.TrustedHeaderFetcher,
	querier *tmquerier.Querier,
	logger *zap.Logger,
	submitter relay.Submitter,
	storage relay.Storage,
	targetChain *relayer.Chain,
	neutronChain *relayer.Chain) *KVProcessor {
	return &KVProcessor{
		trustedHeaderFetcher: trustedHeaderFetcher,
		querier:              querier,
		logger:               logger,
		submitter:            submitter,
		storage:              storage,
		targetChain:          targetChain,
		neutronChain:         neutronChain,
	}
}

// ProcessAndSubmit processes relay.MessageKV. The main method which does all the work of the KVProcessor
func (p *KVProcessor) ProcessAndSubmit(ctx context.Context, m *relay.MessageKV) error {
	latestHeight, err := p.targetChain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to get header for src chain: %w", err)
	}

	proofs, height, err := p.getStorageValues(ctx, uint64(latestHeight), m.KVKeys)
	if err != nil {
		return fmt.Errorf("failed to get storage values with proofs for query_id=%d: %w", m.QueryId, err)
	}
	return p.submitKVWithProof(ctx, int64(height), m.QueryId, proofs)
}

// getStorageValues gets proofs for query type = 'kv'
func (p *KVProcessor) getStorageValues(ctx context.Context, inputHeight uint64, keys neutrontypes.KVKeys) ([]*neutrontypes.StorageValue, uint64, error) {
	stValues := make([]*neutrontypes.StorageValue, 0, len(keys))
	height := uint64(0)

	for _, key := range keys {
		value, h, err := p.querier.QueryTendermintProof(ctx, int64(inputHeight), key.GetPath(), key.GetKey())
		if err != nil {
			return nil, 0, fmt.Errorf("failed to query tendermint proof for path=%s and key=%v: %w", key.GetPath(), key.GetKey(), err)
		}
		stValues = append(stValues, value)
		height = h
	}

	return stValues, height, nil
}

// submitKVWithProof submits the proof for the given query on the given height and tracks the result.
func (p *KVProcessor) submitKVWithProof(
	ctx context.Context,
	height int64,
	queryID uint64,
	proof []*neutrontypes.StorageValue,
) error {
	srcHeader, err := p.getSrcChainHeader(ctx, height)
	if err != nil {
		return fmt.Errorf("failed to get header for height: %d: %w", height, err)
	}

	updateClientMsg, err := p.getUpdateClientMsg(ctx, srcHeader)
	if err != nil {
		return fmt.Errorf("failed to getUpdateClientMsg: %w", err)
	}

	if err = p.submitter.SubmitKVProof(
		ctx,
		uint64(height-1),
		srcHeader.GetHeight().GetRevisionNumber(),
		queryID,
		proof,
		updateClientMsg,
	); err != nil {
		return fmt.Errorf("could not submit proof: %w", err)
	}
	p.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", queryID), zap.Uint64("remote_height", uint64(height-1)), zap.Uint64("trusted_header_height", srcHeader.GetHeight().GetRevisionHeight()))
	return nil
}

func (p *KVProcessor) getSrcChainHeader(ctx context.Context, height int64) (*tmclient.Header, error) {
	var srcHeader *tmclient.Header
	if err := retry.Do(func() error {
		var err error
		srcHeader, err = p.trustedHeaderFetcher.Fetch(ctx, uint64(height))
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		p.logger.Info(
			"failed to GetIBCUpdateHeader", zap.Error(err))
	})); err != nil {
		return nil, err
	}
	return srcHeader, nil
}

func (p *KVProcessor) getUpdateClientMsg(ctx context.Context, srcHeader *tmclient.Header) (sdk.Msg, error) {
	// Construct UpdateClient msg
	var updateMsgRelayer provider.RelayerMessage
	if err := retry.Do(func() error {
		var err error
		updateMsgRelayer, err = p.neutronChain.ChainProvider.MsgUpdateClient(p.neutronChain.PathEnd.ClientID, srcHeader)
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		p.logger.Error(
			"failed to build message", zap.Error(err))
	})); err != nil {
		return nil, err
	}

	updateMsgUnpacked, ok := updateMsgRelayer.(cosmos.CosmosMessage)
	if !ok {
		return nil, errors.New("failed to cast provider.RelayerMessage to cosmos.CosmosMessage")
	}
	return updateMsgUnpacked.Msg, nil
}
