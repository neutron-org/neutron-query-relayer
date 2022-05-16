package proofs

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

// DelegatorDelegations / total stXXX issued
// see cosmos-sdk x/bank/keeper/keeper.go #getSupply
func ProofExchangeRate(ctx context.Context, querier *proofer.ProofQuerier, denom string) error {
	height := int64(0)
	key := append(banktypes.SupplyKey, []byte(denom)...)
	value, err := querier.QueryTendermintProof(ctx, height, banktypes.StoreKey, key)
	if err != nil {
		return fmt.Errorf("error querying exchange rate tendermint proof for denom=%s: %w", denom, err)
	}

	// TODO: use not deprecated types
	var amount sdk.Int
	err = amount.Unmarshal(value.Value)
	if err != nil {
		return fmt.Errorf("error unmarshalling value exchange rate for denom=%s: %w", denom, err)
	}

	fmt.Printf("supply of denom=%s is %+v\n", denom, amount)

	// TODO: do we need to calculate delegations total supply for denom here?

	return nil
}
