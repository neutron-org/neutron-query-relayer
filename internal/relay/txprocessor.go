package relay

import (
	"context"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// Transaction represents single searched tx with height
type Transaction struct {
	Tx     *neutrontypes.TxValue
	Height uint64
}

// TXQuerier fetches transactions from a remote chain with specified txFilter
type TXQuerier interface {
	// SearchTransactions searches for transactions from remote chain.
	// the returned transactions channel can be closed due to one of the following cases:
	// a) All transactions from an RPC call preprocessed successfully
	// b) error encountered during the SearchTransactions method (the error will be written into the returned errs channel)
	// After a txs channel is closed, it's necessary to check the errs channel for a possible errors in a SearchTransactions goroutine
	SearchTransactions(ctx context.Context, query string) (<-chan Transaction, <-chan error)
}

// ChainClient is a minimal interface for tendermint client
type ChainClient interface {
	BlockResults(ctx context.Context, height *int64) (*ctypes.ResultBlockResults, error)
	TxSearch(ctx context.Context, query string, prove bool, page, perPage *int, orderBy string) (*ctypes.ResultTxSearch, error)
}

// TXProcessor precesses transactions from a remote chain and sends them to the neutron
type TXProcessor interface {
	ProcessAndSubmit(ctx context.Context, queryID uint64, tx Transaction, submittedTxsTasksQueue chan PendingSubmittedTxInfo) error
}

// NewErrSubmitTxProofCritical creates a new ErrSubmitTxProofCritical.
func NewErrSubmitTxProofCritical(details error) ErrSubmitTxProofCritical {
	return ErrSubmitTxProofCritical{details: details}
}

// ErrSubmitTxProofCritical is an error type that represents errors critical for the Relayer.
type ErrSubmitTxProofCritical struct {
	// details is the inner error.
	details error
}

// Error implements the error interface.
func (e ErrSubmitTxProofCritical) Error() string {
	return "failed to submit tx proof: " + e.details.Error()
}
