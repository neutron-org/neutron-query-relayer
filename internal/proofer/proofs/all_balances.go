package proofs

import (
	"context"
	"fmt"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

//var queryName = "cosmos.bank.v1beta1.Query/AllBalances"

// cosmos-sdk x/bank/keeper/querier.go
// ModuleName = "bank"

//x/bank/types/key.go

// TODO: use real cosmos-sdk all balances struct here?
type AllBalancesResponse struct {
	Balances []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"balances"`
	Pagination struct {
		Total string `json:"total"`
	} `json:"pagination"`
}

func ProofAllBalances(ctx context.Context, address string, denom string, querier *proofer.QueryKeyProofer) (map[string]string, error) {
	inputHeight := int64(0)
	storeKey := banktypes.StoreKey
	bz, err := cosmostypes.GetFromBech32(address, "terra")
	if err != nil {
		return nil, err
	}

	key := append(banktypes.CreateAccountBalancesPrefix(bz), []byte(denom)...)
	fmt.Printf("About to querier.QueryTendermintProof")
	value, height, err := querier.QueryTendermintProof(ctx, querier.ChainID, inputHeight, storeKey, key)
	if err != nil {
		return nil, err
	}
	fmt.Printf("QueryTendermintProof worked")

	var amount cosmostypes.Coin
	if err := amount.Unmarshal(value.Value); err != nil {
		fmt.Println("err: %s", err)
		return nil, err
	}
	fmt.Printf("\nCoin: %v, Height %v Err %v", amount, height, err)

	return nil, nil
}

func ProofAllDelegations(ctx context.Context, validators []string, delegator string, querier *proofer.QueryKeyProofer) (map[string]string, error) {
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

func ProofAllDelegations2(ctx context.Context, delegator string, querier *proofer.QueryKeyProofer) (map[string]string, error) {
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

// TODO: rewards
// TODO: transactions
