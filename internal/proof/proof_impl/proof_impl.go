package proof_impl

import "github.com/lidofinance/cosmos-query-relayer/internal/proof"

type ProoferImpl struct {
	querier *proof.Querier
}

func NewProofer(querier *proof.Querier) proof.Proofer {
	return ProoferImpl{querier: querier}
}
