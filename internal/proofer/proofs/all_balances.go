package proofs

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

// ProofAllBalances gets proofs for query type = 'x/bank/GetAllBalances'
func ProofAllBalances(ctx context.Context, querier *proofer.ProofQuerier, chainPrefix string, address string, denom string) ([]*proofer.StorageValue, uint64, error) {
	storeKey := banktypes.StoreKey
	bytesAddress, err := sdk.GetFromBech32(address, chainPrefix)
	if err != nil {
		return nil, 0, err
	}

	key := append(banktypes.CreateAccountBalancesPrefix(bytesAddress), []byte(denom)...)
	value, height, err := querier.QueryTendermintProof(ctx, int64(0), storeKey, key)
	if err != nil {
		fmt.Printf("failed to query tendermint proof for balances: %s", err)
		return nil, 0, fmt.Errorf("failed to query tendermint proof for balances: %w", err)
	}

	var amount sdk.Coin
	if err := amount.Unmarshal(value.Value); err != nil {
		fmt.Printf("failed to unmarshal the balances response: %s", err)
		return nil, 0, err
	}
	fmt.Printf("\nCoin: %+v, Err %v\n", amount, err)

	return []*proofer.StorageValue{value}, height, err
}
