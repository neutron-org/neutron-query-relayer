package proofer

import (
	"context"
	"fmt"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
)

func ProofEvent(ctx context.Context, event coretypes.ResultEvent) {
	// TODO
	queries := ExpandEvent(event)
	ProofQueries(ctx, queries)
}

func ProofQueries(ctx context.Context, queries []Query) (map[string]string, error) {
	for _, query := range queries {
		value, _, err := ProofQuery(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("could not get proof for a query with id = %s: %w", query.QueryId, err)
		}

		fmt.Printf("\nProof for a query: %+v", value)
	}

	// TODO
	return nil, nil
}

func ProofQuery(ctx context.Context, query Query) ([]StorageValue, *clienttypes.Height, error) {
	// take the query with its data
	// for this query, query target blockchain with its proofs (QueryTendermintProof)
	//ccc, logger, homepath := GetChainConfig()
	querier, err := NewQueryKeyProofer("addr-todo", query.ChainId)

	// FIXME: need somehow fixed height at the end
	var inputHeight int64 = 0
	key := []byte("todo")
	storeKey := "TODO"

	value, height, err := querier.QueryTendermintProof(ctx, query.ChainId, inputHeight, storeKey, key)

	fmt.Printf("Height: %s", height)

	if err != nil {
		return nil, nil, fmt.Errorf("could not create proof querier: %w", err)
	}

	// fixme
	return []StorageValue{*value}, &height, nil
}
