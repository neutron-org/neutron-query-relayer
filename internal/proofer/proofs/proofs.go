package proofs

import (
	"context"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

type Proofer interface {
	GetDelegatorDelegations(ctx context.Context, prefix string, delegator string) ([]proofer.StorageValue, uint64, error)
	GetBalance(ctx context.Context, chainPrefix string, addr string, denom string) ([]proofer.StorageValue, uint64, error)
	RecipientTransactions(ctx context.Context, queryParams map[string]string) ([]proofer.CompleteTransactionProof, error)
}

type ProoferImpl struct {
	querier *proofer.ProofQuerier
}

func NewProofer(querier *proofer.ProofQuerier) Proofer {
	return ProoferImpl{querier}
}
