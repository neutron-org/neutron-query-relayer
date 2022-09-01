package proof_impl

import (
	"context"
	"fmt"
	neutronmetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	"github.com/neutron-org/cosmos-query-relayer/internal/relay"
	"go.uber.org/zap"
	"time"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// GetStorageValues gets proofs for query type = 'kv'
func (p ProoferImpl) GetStorageValues(ctx context.Context, inputHeight uint64, keys neutrontypes.KVKeys) ([]*neutrontypes.StorageValue, uint64, error) {
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

func (p ProoferImpl) ProcessMessageKV(ctx context.Context, m *relay.MessageKV) error {
	r.logger.Debug("running processMessageKV for msg", zap.Uint64("query_id", m.QueryId))
	latestHeight, err := r.targetChain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to get header for src chain: %w", err)
	}

	ok, err := r.isQueryOnTime(m.QueryId, uint64(latestHeight))
	if err != nil || !ok {
		return fmt.Errorf("error on checking previous query update with query_id=%d: %w", m.QueryId, err)
	}

	proofs, height, err := r.proofer.GetStorageValues(ctx, uint64(latestHeight), m.KVKeys)
	if err != nil {
		return fmt.Errorf("failed to get storage values with proofs for query_id=%d: %w", m.QueryId, err)
	}
	return r.submitProof(ctx, int64(height), m.QueryId, proofs)
}

// isQueryOnTime checks if query satisfies update period condition which is set by RELAYER_KV_UPDATE_PERIOD env, also modifies storage w last block
func (r *ProoferImpl) isQueryOnTime(queryID uint64, currentBlock uint64) (bool, error) {
	// if it wasn't set in config
	if r.cfg.MinKvUpdatePeriod == 0 {
		return true, nil
	}

	previous, ok, err := r.storage.GetLastQueryHeight(queryID)
	if err != nil {
		return false, err
	}

	if !ok || previous+r.cfg.MinKvUpdatePeriod <= currentBlock {
		err := r.storage.SetLastQueryHeight(queryID, currentBlock)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, fmt.Errorf("attempted to update query results too soon: last update was on block=%d, current block=%d, maximum update period=%d", previous, currentBlock, r.cfg.MinKvUpdatePeriod)
}

// submitProof submits the proof for the given query on the given height and tracks the result.
func (r *ProoferImpl) submitProof(
	ctx context.Context,
	height int64,
	queryID uint64,
	proof []*neutrontypes.StorageValue,
) error {
	srcHeader, err := r.getSrcChainHeader(ctx, height)
	if err != nil {
		return fmt.Errorf("failed to get header for height: %d: %w", height, err)
	}

	updateClientMsg, err := r.getUpdateClientMsg(ctx, srcHeader)
	if err != nil {
		return fmt.Errorf("failed to getUpdateClientMsg: %w", err)
	}

	st := time.Now()
	if err = r.submitter.SubmitKVProof(
		ctx,
		uint64(height-1),
		srcHeader.GetHeight().GetRevisionNumber(),
		queryID,
		r.cfg.AllowKVCallbacks,
		proof,
		updateClientMsg,
	); err != nil {
		neutronmetrics.AddFailedProof(string(neutrontypes.InterchainQueryTypeKV), time.Since(st).Seconds())
		return fmt.Errorf("could not submit proof: %w", err)
	}
	neutronmetrics.AddSuccessProof(string(neutrontypes.InterchainQueryTypeKV), time.Since(st).Seconds())
	r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", queryID))
	return nil
}
