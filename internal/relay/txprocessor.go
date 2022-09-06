package relay

import (
	"context"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// Transaction represents single searched tx with height
type Transaction struct {
	Tx     *neutrontypes.TxValue
	Height uint64
}

// TXQuerier fetches transactions from a remote chain with specified txFilter
type TXQuerier interface {
	SearchTransactions(ctx context.Context, txFilter neutrontypes.TransactionsFilter) <-chan Transaction
	Err() error
}

// ChainClient is a minimal interface for tendermint client
type ChainClient interface {
	BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error)
	TxSearch(ctx context.Context, query string, prove bool, page, perPage *int, orderBy string) (*ctypes.ResultTxSearch, error)
}

// TXProcessor precesses transactions from a remote chain and sends them to the neutron
type TXProcessor interface {
	ProcessAndSubmit(ctx context.Context, queryID uint64, txs <-chan Transaction) (uint64, error)
	GetSubmitNotificationChannel() <-chan PendingSubmittedTxInfo
}