package proofs

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

// GetBalance gets proofs for query type = 'x/bank/GetBalance'
func (p ProoferImpl) GetBalance(ctx context.Context, chainPrefix string, addr string, denom string) ([]proofer.StorageValue, uint64, error) {
	storeKey := banktypes.StoreKey
	bytesAddress, err := sdk.GetFromBech32(addr, chainPrefix)
	if err != nil {
		return nil, 0, err
	}

	key := append(banktypes.CreateAccountBalancesPrefix(bytesAddress), []byte(denom)...)
	value, height, err := p.querier.QueryTendermintProof(ctx, int64(0), storeKey, key)
	if err != nil {
		fmt.Printf("failed to query tendermint proof for balances: %s", err)
		return nil, 0, fmt.Errorf("failed to query tendermint proof for balances: %w", err)
	}

	return []proofer.StorageValue{*value}, height, err
}

func parseGetBalanceValue(value proofer.StorageValue) {
	var amount sdk.Coin
	if err := amount.Unmarshal(value.Value); err != nil {
		fmt.Printf("failed to unmarshal the balances response: %s", err)
		return
	}
	fmt.Printf("\nCoin: %+v", amount)
}
