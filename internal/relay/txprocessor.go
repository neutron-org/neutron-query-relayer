package relay

import (
	"context"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

type TXQuerier interface {
	SearchTransactions(ctx context.Context, txFilter neutrontypes.TransactionsFilter) <-chan Transaction
	Err() error
}

type ChainClient interface {
	BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error)
	TxSearch(ctx context.Context, query string, prove bool, page, perPage *int, orderBy string) (*ctypes.ResultTxSearch, error)
}

type TXProcessor interface {
	ProcessAndSubmit(ctx context.Context, queryID uint64, txs <-chan Transaction) (uint64, error)
}
