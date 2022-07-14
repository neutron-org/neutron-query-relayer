package relay

import (
	"context"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// Proofer can obtain proofs for different kinds of queries we need answers to
type Proofer interface {
	GetDelegatorDelegations(ctx context.Context, height uint64, prefix string, delegator string) ([]proof.StorageValue, uint64, error)
	GetBalance(ctx context.Context, height uint64, chainPrefix string, addr string, denom string) ([]proof.StorageValue, uint64, error)
	GetSupply(ctx context.Context, height uint64, denom string) ([]proof.StorageValue, uint64, error)
	RecipientTransactions(ctx context.Context, queryParams map[string]string) (map[uint64][]*neutrontypes.TxValue, error)
}
