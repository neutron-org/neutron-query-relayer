package proofs

import (
	"context"
	"fmt"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
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

//type Balances

func ProofAllBalances(ctx context.Context, address string, querier *proofer.QueryKeyProofer) (map[string]string, error) {
	inputHeight := int64(0)
	storeKey := banktypes.StoreKey
	bz, err := cosmostypes.GetFromBech32(address, "terra")
	if err != nil {
		return nil, err
	}

	denoms := []string{"uluna", "uusd", "ukek"}

	for _, denom := range denoms {
		fmt.Println("processing denom...")
		key := append(banktypes.CreateAccountBalancesPrefix(bz), []byte(denom)...)
		value, proof, height, err := querier.QueryTendermintProof(ctx, querier.ChainID, inputHeight, storeKey, key)

		var amount cosmostypes.Coin
		if err := amount.Unmarshal(value); err != nil {
			fmt.Println("err: %s", err)
			return nil, err
		}
		fmt.Printf("\nValue: %v, Proof %p Height %v Err %w", amount, proof, height, err)
	}

	return nil, nil
}
