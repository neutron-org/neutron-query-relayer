package proofs

import (
	"context"
	"fmt"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

func ProofAllDelegations(ctx context.Context, validators []string, delegator string, querier *proofer.ProofQuerier) (map[string]string, error) {
	inputHeight := int64(0)
	storeKey := stakingtypes.StoreKey
	delegatorBz, err := cosmostypes.GetFromBech32(delegator, "terra")
	if err != nil {
		return nil, err
	}

	//delegatorPrefixKey := stakingtypes.GetDelegationsKey(delegatorBz)
	for _, validator := range validators {
		validatorBz, err := cosmostypes.GetFromBech32(validator, "terravaloper")
		if err != nil {
			return nil, err
		}

		key := stakingtypes.GetDelegationKey(delegatorBz, validatorBz)
		fmt.Println("processing delegation...")
		//key := append(banktypes.CreateAccountBalancesPrefix(bz), []byte(denom)...)
		value, height, err := querier.QueryTendermintProof(ctx, querier.ChainID, inputHeight, storeKey, key)

		var delegation stakingtypes.Delegation
		if err := delegation.Unmarshal(value.Value); err != nil {
			fmt.Println("err: %s", err)
			return nil, err
		}
		fmt.Printf("\nDelegation:\n %v, Height %v Err %v", delegation, height, err)
	}

	return nil, nil
}

func ProofAllDelegations2(ctx context.Context, delegator string, querier *proofer.ProofQuerier) (map[string]string, error) {
	inputHeight := int64(0)
	storeKey := stakingtypes.StoreKey
	delegatorBz, err := cosmostypes.GetFromBech32(delegator, "terra")
	if err != nil {
		return nil, err
	}

	delegatorPrefixKey := stakingtypes.GetDelegationsKey(delegatorBz)
	result, height, err := querier.QueryIterateTendermintProof(ctx, querier.ChainID, inputHeight, storeKey, delegatorPrefixKey)
	if err != nil {
		fmt.Printf("\nerr: %s", err)
		return nil, err
	}
	fmt.Printf("\nResult: %+v\nHeight: %+v", result, height)

	return nil, nil
}
