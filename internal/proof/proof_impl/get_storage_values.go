package proof_impl

import (
	"context"
	"fmt"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// GetStorageValuesWithProof gets proofs for query type = 'kv'
func (p ProoferImpl) GetStorageValuesWithProof(ctx context.Context, inputHeight uint64, keys neutrontypes.KVKeys) ([]*neutrontypes.StorageValue, uint64, error) {
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
