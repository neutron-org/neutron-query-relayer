package proofer

import (
	"github.com/tendermint/tendermint/rpc/coretypes"
)

func TryProof(event coretypes.ResultEvent) (map[string]string, error) {
	types := event.Events["message.type"]
	// TODO
	return nil, nil

	for i := 0; i < items; i++ {
		query_params := parseParams(params, queryIds[i])
		queries = append(queries, Query{source[0], connections[i], chains[i], queryIds[i], types[i], query_params})
	}
}

func Proof
