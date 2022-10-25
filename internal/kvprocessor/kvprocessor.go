package kvprocessor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"go.uber.org/zap"

	neutronmetrics "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron-query-relayer/internal/tmquerier"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// KVProcessor is implementation of relay.KVProcessor that processes event query KV type.
// Obtains the proof for a query we need to process, and sends it to  the neutron
type KVProcessor struct {
	querier           *tmquerier.Querier
	minKVUpdatePeriod uint64
	logger            *zap.Logger
	submitter         relay.Submitter
	storage           relay.Storage
	targetChain       *relayer.Chain
	neutronChain      *relayer.Chain
}

func NewKVProcessor(
	querier *tmquerier.Querier,
	minKVUpdatePeriod uint64,
	logger *zap.Logger,
	submitter relay.Submitter,
	storage relay.Storage,
	targetChain *relayer.Chain,
	neutronChain *relayer.Chain) relay.KVProcessor {
	return &KVProcessor{
		querier:           querier,
		minKVUpdatePeriod: minKVUpdatePeriod,
		logger:            logger,
		submitter:         submitter,
		storage:           storage,
		targetChain:       targetChain,
		neutronChain:      neutronChain,
	}
}

// ProcessAndSubmit processes relay.MessageKV. The main method which does all the work of the KVProcessor
func (p *KVProcessor) ProcessAndSubmit(m *relay.MessageKV) error {
	latestHeight, err := p.targetChain.ChainProvider.QueryLatestHeight(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get header for src chain: %w", err)
	}

	ok, err := p.isQueryOnTime(m.QueryId, uint64(latestHeight))
	if err != nil || !ok {
		return fmt.Errorf("error on checking previous query update with query_id=%d: %w", m.QueryId, err)
	}

	proofs, height, err := p.getStorageValues(uint64(latestHeight), m.KVKeys)
	if err != nil {
		return fmt.Errorf("failed to get storage values with proofs for query_id=%d: %w", m.QueryId, err)
	}
	return p.submitKVWithProof(int64(height), m.QueryId, proofs)
}

// getStorageValues gets proofs for query type = 'kv'
func (p *KVProcessor) getStorageValues(inputHeight uint64, keys neutrontypes.KVKeys) ([]*neutrontypes.StorageValue, uint64, error) {
	stValues := make([]*neutrontypes.StorageValue, 0, len(keys))
	height := uint64(0)

	for _, key := range keys {
		value, h, err := p.querier.QueryTendermintProof(int64(inputHeight), key.GetPath(), key.GetKey())
		if err != nil {
			return nil, 0, fmt.Errorf("failed to query tendermint proof for path=%s and key=%v: %w", key.GetPath(), key.GetKey(), err)
		}
		stValues = append(stValues, value)
		height = h
	}

	return stValues, height, nil
}

// isQueryOnTime checks if query satisfies update period condition which is set by RELAYER_KV_UPDATE_PERIOD env, also modifies storage w last block
func (p *KVProcessor) isQueryOnTime(queryID uint64, currentBlock uint64) (bool, error) {
	// if it wasn't set in config
	if p.minKVUpdatePeriod == 0 {
		return true, nil
	}

	previous, err := p.storage.GetLastQueryHeight(queryID)
	if err != nil {
		return false, err
	}

	if previous+p.minKVUpdatePeriod <= currentBlock {
		err := p.storage.SetLastQueryHeight(queryID, currentBlock)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, fmt.Errorf("attempted to update query results too soon: last update was on block=%d, current block=%d, maximum update period=%d", previous, currentBlock, p.minKVUpdatePeriod)
}

// submitKVWithProof submits the proof for the given query on the given height and tracks the result.
func (p *KVProcessor) submitKVWithProof(
	height int64,
	queryID uint64,
	proof []*neutrontypes.StorageValue,
) error {
	srcHeader, err := p.getSrcChainHeader(height)
	if err != nil {
		return fmt.Errorf("failed to get header for height: %d: %w", height, err)
	}

	updateClientMsg, err := p.getUpdateClientMsg(srcHeader)
	if err != nil {
		return fmt.Errorf("failed to getUpdateClientMsg: %w", err)
	}

	st := time.Now()
	if err = p.submitter.SubmitKVProof(
		uint64(height-1),
		srcHeader.GetHeight().GetRevisionNumber(),
		queryID,
		proof,
		updateClientMsg,
	); err != nil {
		neutronmetrics.AddFailedProof(string(neutrontypes.InterchainQueryTypeKV), time.Since(st).Seconds())
		return fmt.Errorf("could not submit proof: %w", err)
	}
	neutronmetrics.AddSuccessProof(string(neutrontypes.InterchainQueryTypeKV), time.Since(st).Seconds())
	p.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", queryID))
	return nil
}

func (p *KVProcessor) getSrcChainHeader(height int64) (ibcexported.Header, error) {
	start := time.Now()
	var srcHeader ibcexported.Header
	if err := retry.Do(func() error {
		var err error
		srcHeader, err = p.targetChain.ChainProvider.GetIBCUpdateHeader(context.Background(), height, p.neutronChain.ChainProvider, p.neutronChain.PathEnd.ClientID)
		return err
	}, relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		p.logger.Info(
			"failed to GetIBCUpdateHeader", zap.Error(err))
	})); err != nil {
		neutronmetrics.RecordActionDuration("GetIBCUpdateHeader", time.Since(start).Seconds())
		return nil, err
	}
	neutronmetrics.RecordActionDuration("GetIBCUpdateHeader", time.Since(start).Seconds())
	return srcHeader, nil
}

func (p *KVProcessor) getUpdateClientMsg(srcHeader ibcexported.Header) (sdk.Msg, error) {
	start := time.Now()
	// Construct UpdateClient msg
	var updateMsgRelayer provider.RelayerMessage
	if err := retry.Do(func() error {
		var err error
		updateMsgRelayer, err = p.neutronChain.ChainProvider.UpdateClient(p.neutronChain.PathEnd.ClientID, srcHeader)
		return err
	}, relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		p.logger.Error(
			"failed to build message", zap.Error(err))
	})); err != nil {
		return nil, err
	}

	updateMsgUnpacked, ok := updateMsgRelayer.(cosmos.CosmosMessage)
	if !ok {
		return nil, errors.New("failed to cast provider.RelayerMessage to cosmos.CosmosMessage")
	}
	neutronmetrics.RecordActionDuration("GetUpdateClientMsg", time.Since(start).Seconds())
	return updateMsgUnpacked.Msg, nil
}
