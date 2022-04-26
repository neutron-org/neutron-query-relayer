package proofer

import (
	"context"
	"fmt"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"strings"
)

type Query struct {
	SourceChainId string
	ConnectionId  string
	ChainId       string
	QueryId       string
	Type          string
	Params        map[string]string
}

func TryProofEvent(ctx context.Context, event coretypes.ResultEvent) (map[string]string, error) {
	source := event.Events["source"]
	connections := event.Events["message.connection_id"]
	chains := event.Events["message.chain_id"]
	queryIds := event.Events["message.query_id"]
	types := event.Events["message.type"]
	params := event.Events["message.parameters"]

	items := len(queryIds)

	queries := make([]Query, 0, len(queryIds))
	for i := 0; i < items; i++ {
		queryParams := parseParams(params, queryIds[i])
		queries = append(queries, Query{source[0], connections[i], chains[i], queryIds[i], types[i], queryParams})
	}

	for _, query := range queries {
		value, proof, _, err := ProofQuery(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("could not get proof for a query with id = %s: %w", query.QueryId, err)
		}

		fmt.Printf("\nProof for a query: %+v\nValue: %+v", proof, value)
	}

	return nil, nil
}

func ProofQuery(ctx context.Context, query Query) ([]byte, []byte, *clienttypes.Height, error) {
	// take the query with its data
	// for this query, query target blockchain with its proofs (QueryTendermintProof)
	ccc, logger, homepath := GetChainConfig(query)
	querier, err := NewProofQueries(logger, homepath, ccc)

	// FIXME: need somehow fixed height at the end
	var inputHeight int64 = 0
	key := []byte("todo")

	value, proof, height, err := querier.QueryTendermintProof(ctx, query.ChainId, inputHeight, key)

	fmt.Printf("Height: %s", height)

	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not create proof querier: %w", err)
	}

	return value, proof, &height, nil
}

func parseParams(params []string, query_id string) map[string]string {
	out := map[string]string{}
	for _, p := range params {
		if strings.HasPrefix(p, query_id) {
			parts := strings.SplitN(p, ":", 3)
			out[parts[1]] = parts[2]
		}
	}
	return out
}
