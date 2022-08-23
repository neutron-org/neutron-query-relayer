package relay

import (
	"context"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// TransactionSlice represents single searched tx with height
type TransactionSlice struct {
	Tx     *neutrontypes.TxValue
	Height uint64
}

// Proofer can obtain proofs for different kinds of queries we need answers to
type Proofer interface {
	GetStorageValues(context.Context, uint64, neutrontypes.KVKeys) ([]*neutrontypes.StorageValue, uint64, error)
	SearchTransactions(context.Context, map[string]string) (txs []TransactionSlice, err error)
}
