package relay

import (
	"time"
)

// PendingSubmittedTxInfo contains information about transaction witch were submitted but have to be confirmed(committed or not)
type PendingSubmittedTxInfo struct {
	// QueryID is the query_id transactions was submitted for
	QueryID uint64
	// SubmittedTxHash is the hash of the *remote fetched transaction* was submitted
	SubmittedTxHash string
	// NeutronHash is the hash of the *neutron chain transaction* witch responsible for delivering remote transaction to neutron
	NeutronHash string
	// SubmitTime is the time when the remote transaction was submitted to the neutron chain
	SubmitTime time.Time
}

// SubmittedTxInfo is a struct witch contains status of fetched and submitted transaction
type SubmittedTxInfo struct {
	// SubmittedTxStatus is a status of a processing state
	Status SubmittedTxStatus
	// Message is some additional information witch can be useful, e.g. error message for ErrorOnSubmit and ErrorOnCommit statuses
	Message string
}

type SubmittedTxStatus string

const (
	// Submitted describes successfully submitted tx (temporary status, should be clarified into Committed or ErrorOnCommit)
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
	GetLastQueryHeight(queryID uint64) (block uint64, exists bool, err error)
	SetLastQueryHeight(queryID uint64, block uint64) error
	SetTxStatus(queryID uint64, hash string, neutronHash string, status SubmittedTxInfo) (err error)
	TxExists(queryID uint64, hash string) (exists bool, err error)
	Close() error
}
