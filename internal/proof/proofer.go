package proof

import (
	"context"
)

type Proofer interface {
	GetDelegatorDelegations(ctx context.Context, prefix string, delegator string) ([]StorageValue, uint64, error)
	GetBalance(ctx context.Context, chainPrefix string, addr string, denom string) ([]StorageValue, uint64, error)
	RecipientTransactions(ctx context.Context, queryParams map[string]string) ([]TxValue, error)
}
