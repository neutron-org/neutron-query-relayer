package proofer

import (
	"github.com/tendermint/tendermint/rpc/coretypes"
)

type Query struct {
	SourceChainId string
	ConnectionId  string
	ChainId       string
	QueryId       string
	Type          string
	Params        map[string]string
}

func ProofQuery(query Query) (map[string]string, error) {
	// take the query with its data
	// for this query, query target blockchain with its proofs (QueryTendermintProof)

	return nil, nil
}

func TryProofEvent(event coretypes.ResultEvent) (map[string]string, error) {
	//types := event.Events["message.type"]
	//// TODO
	//return nil, nil
	//
	//for i := 0; i < items; i++ {
	//	query_params := parseParams(params, queryIds[i])
	//	queries = append(queries, Query{source[0], connections[i], chains[i], queryIds[i], types[i], query_params})
	//}
	return nil, nil
}
