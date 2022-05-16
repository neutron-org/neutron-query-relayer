package proofs

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

// GetDelegatorDelegations gets proofs for query type = 'x/staking/GetDelegatorDelegations'
func GetDelegatorDelegations(ctx context.Context, querier *proofer.ProofQuerier, prefix string, delegator string) ([]proofer.StorageValue, error) {
	inputHeight := int64(0)
	storeKey := stakingtypes.StoreKey
	delegatorBz, err := sdk.GetFromBech32(delegator, prefix)
	if err != nil {
		return nil, err
	}

	delegatorPrefixKey := stakingtypes.GetDelegationsKey(delegatorBz)
	result, err := querier.QueryIterateTendermintProof(ctx, inputHeight, storeKey, delegatorPrefixKey)

	return result, err
}
