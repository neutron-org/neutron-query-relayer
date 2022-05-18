package proofs

import (
	"context"
	"fmt"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

// GetSupply gets proofs for query type = 'x/bank/GetSupply'
// Needed for proof of stXXX rate (?) = DelegatorDelegations / total stXXX issued
func GetSupply(ctx context.Context, querier *proofer.ProofQuerier, denom string) ([]proofer.StorageValue, uint64, error) {
	key := append(banktypes.SupplyKey, []byte(denom)...)
	value, height, err := querier.QueryTendermintProof(ctx, 0, banktypes.StoreKey, key)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying exchange rate tendermint proof for denom=%s: %w", denom, err)
	}

	//var amount sdk.Int
	//err = amount.Unmarshal(value.Value)
	//if err != nil {
	//	return nil, 0, fmt.Errorf("error unmarshalling value exchange rate for denom=%s: %w", denom, err)
	//}
	//
	//fmt.Printf("supply of denom=%s is %+v\n", denom, amount)

	// TODO: do we need to calculate delegations total supply for denom here?

	return []proofer.StorageValue{*value}, height, nil
}
