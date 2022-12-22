package relay

import (
	"time"
)

// PendingSubmittedTxInfo contains information about transaction which was submitted but has to be confirmed (committed or not)
type PendingSubmittedTxInfo struct {
	// QueryID is the query_id transactions was submitted for
	QueryID uint64 `json:"query_id"`
	// SubmittedTxHash is the hash of a transaction we fetched from the remote chain
	SubmittedTxHash string `json:"submitted_tx_hash"`
	// NeutronHash is the hash of the *neutron chain transaction* which is responsible for delivering remote transaction to neutron
	NeutronHash string `json:"neutron_hash"`
}

type UnsuccessfulTxInfo struct {
	// QueryID is the query_id transactions was submitted for
	QueryID uint64 `json:"query_id"`
	// SubmittedTxHash is the hash of a transaction we fetched from the remote chain
	SubmittedTxHash string `json:"submitted_tx_hash"`
	// NeutronHash is the hash of the *neutron chain transaction* which is responsible for delivering remote transaction to neutron
	NeutronHash string `json:"neutron_hash"`
	// ErrorTime is the time when the error was added
	ErrorTime time.Time `json:"error_time"`
	// Status is the status of unsuccessful tx
	Status SubmittedTxStatus `json:"status"`
	// Message is the more descriptive message for the error
	Message string `json:"message"`
}

// SubmittedTxInfo is a struct which contains status of fetched and submitted transaction
type SubmittedTxInfo struct {
	// SubmittedTxStatus is a status of a processing state
	Status SubmittedTxStatus `json:"status"`
	// Message is some additional information which can be useful, e.g. error message for ErrorOnSubmit and ErrorOnCommit statuses
	Message string `json:"message"`
}

type SubmittedTxStatus string

const (
	// Submitted describes successfully submitted tx (temporary status, should be eventually replaced with Committed or ErrorOnCommit)
	Submitted SubmittedTxStatus = "Submitted"
	// ErrorOnSubmit describes error during submit operation
	ErrorOnSubmit SubmittedTxStatus = "ErrorOnSubmit"
	// Committed describes tx successfully committed into a block
	Committed SubmittedTxStatus = "Committed"
	// ErrorOnCommit describes error during commit operation
	ErrorOnCommit SubmittedTxStatus = "ErrorOnCommit"
)

// Storage is local storage we use to store queries history: known queries, know transactions and its statuses
type Storage interface {
	GetAllPendingTxs() ([]*PendingSubmittedTxInfo, error)
	GetAllUnsuccessfulTxs() ([]*UnsuccessfulTxInfo, error)
	GetCachedTx(queryID uint64, hash string) (*Transaction, error)
	GetLastQueryHeight(queryID uint64) (block uint64, found bool, err error)
	SetLastQueryHeight(queryID uint64, block uint64) error
	SetTxStatus(queryID uint64, hash string, neutronHash string, status SubmittedTxInfo, processedTx *Transaction) (err error)
	TxExists(queryID uint64, hash string) (exists bool, err error)
	Close() error
}
