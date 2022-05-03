package proofer

import (
	"github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

func GetProof(event coretypes.ResultEvent) {
	// TODO: proof event, submit transaction back to lido chain (async?)
	queries := EventToQueries(event)
	proof, err := CollectProof(queries)

	if err != nil {
		// TODO: error logging, metrics
	}

	err = SubmitProof(proof)
	if err != nil {
		// TODO: error logging, metrics
	}
}

func EventToQueries(event coretypes.ResultEvent) []Query {
	return []Query{}
}

func CollectProof([]Query) (Proof, error) {
	return Proof{}, nil
}

func SubmitProof(proof Proof) error {
	return chain.SubmitTransaction()
}
