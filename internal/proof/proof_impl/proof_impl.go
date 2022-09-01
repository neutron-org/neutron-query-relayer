package proof_impl

import (
	"github.com/neutron-org/neutron-query-relayer/internal/proof"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
)

type ProoferImpl struct {
	querier           *proof.Querier
	minKVUpdatePeriod uint64
	allowKVCallbacks  bool
}

func NewProofer(querier *proof.Querier) relay.Proofer {
	return ProoferImpl{querier: querier}
}
