package relay

import (
	"context"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// Transaction represents single searched tx with height
type Transaction struct {
	Tx     *neutrontypes.TxValue
	Height uint64
}

// Proofer can obtain proofs for different kinds of queries we need answers to
type Proofer interface {
	GetStorageValues(context.Context, uint64, neutrontypes.KVKeys) ([]*neutrontypes.StorageValue, uint64, error)
	SearchTransactions(context.Context, []neutrontypes.FilterItem) (txs []Transaction, err error)
}
