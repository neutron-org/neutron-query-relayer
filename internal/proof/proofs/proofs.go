package proofs

import (
	"context"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
)

type Proofer interface {
	GetDelegatorDelegations(ctx context.Context, prefix string, delegator string) ([]proof.StorageValue, uint64, error)
	GetBalance(ctx context.Context, chainPrefix string, addr string, denom string) ([]proof.StorageValue, uint64, error)
	RecipientTransactions(ctx context.Context, queryParams map[string]string) ([]proof.CompleteTransactionProof, error)
}

type ProoferImpl struct {
	querier *proof.Querier
}

func NewProofer(querier *proof.Querier) Proofer {
	return ProoferImpl{querier}
}
