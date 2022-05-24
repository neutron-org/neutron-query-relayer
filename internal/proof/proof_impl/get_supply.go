package proof_impl

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
)

// GetSupply gets proofs for query type = 'x/bank/GetSupply'
// Needed as part of a proof of stXXX rate (?) = DelegatorDelegations / total stXXX issued
func (p ProoferImpl) GetSupply(ctx context.Context, inputHeight uint64, denom string) ([]proof.StorageValue, uint64, error) {
	key := append(banktypes.SupplyKey, []byte(denom)...)
	value, height, err := p.querier.QueryTendermintProof(ctx, int64(inputHeight), banktypes.StoreKey, key)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying exchange rate tendermint proof for denom=%s: %w", denom, err)
	}

	return []proof.StorageValue{*value}, height, nil
}

func parseGetSupplyValue(value proof.StorageValue) {
	var amount sdk.Int
	err := amount.Unmarshal(value.Value)
	if err != nil {
		return
	}

	fmt.Printf("supply is %+v\n", amount)
}
