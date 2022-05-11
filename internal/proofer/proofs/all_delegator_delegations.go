package proofs

import (
	"context"
	"fmt"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

func ProofAllDelegations(ctx context.Context, querier *proofer.ProofQuerier, prefix string, validators []string, delegator string) (map[string]string, error) {
	inputHeight := int64(0)
	storeKey := stakingtypes.StoreKey
	delegatorBz, err := cosmostypes.GetFromBech32(delegator, prefix)
	if err != nil {
		return nil, err
	}

	for _, validator := range validators {
		validatorBz, err := cosmostypes.GetFromBech32(validator, prefix+cosmostypes.PrefixValidator+cosmostypes.PrefixOperator)
		if err != nil {
			return nil, err
		}

		key := stakingtypes.GetDelegationKey(delegatorBz, validatorBz)
		value, err := querier.QueryTendermintProof(ctx, inputHeight, storeKey, key)

		var delegation stakingtypes.Delegation
		if err := delegation.Unmarshal(value.Value); err != nil {
			fmt.Printf("failed to unmarshal delegations: %s\n", err)
			return nil, fmt.Errorf("failed to unmarshal delegations: %w", err)
		}
		fmt.Printf("\nDelegation:\n %v, Err %v", delegation, err)
	}

	return nil, nil
}

func ProofAllDelegations2(ctx context.Context, querier *proofer.ProofQuerier, prefix string, delegator string) (map[string]string, error) {
	inputHeight := int64(0)
	storeKey := stakingtypes.StoreKey
	delegatorBz, err := cosmostypes.GetFromBech32(delegator, prefix)
	if err != nil {
		return nil, err
	}

	delegatorPrefixKey := stakingtypes.GetDelegationsKey(delegatorBz)
	result, err := querier.QueryIterateTendermintProof(ctx, inputHeight, storeKey, delegatorPrefixKey)
	if err != nil {
		fmt.Printf("\nfailed to fetch tendermint proof: %s", err)
		return nil, fmt.Errorf("failed to fetch tendermint proof: %w", err)
	}
	fmt.Printf("\nResult: %+v", result)

	return nil, nil
}
