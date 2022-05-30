package proof_impl

import (
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	"github.com/lidofinance/cosmos-query-relayer/internal/relay"
)

type ProoferImpl struct {
	querier *proof.Querier
}

func NewProofer(querier *proof.Querier) relay.Proofer {
	return ProoferImpl{querier: querier}
}
