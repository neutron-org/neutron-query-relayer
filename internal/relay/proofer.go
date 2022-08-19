package relay

import (
	"context"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// Proofer can obtain proofs for different kinds of queries we need answers to
type Proofer interface {
	GetStorageValues(context.Context, uint64, neutrontypes.KVKeys) ([]*neutrontypes.StorageValue, uint64, error)
	SearchTransactions(context.Context, RecipientTransactionsParams) (map[uint64][]*neutrontypes.TxValue, error)
}
