package relay

import (
	"time"
)

type SubmittedTxStatus struct {
	Status  string
	Message string
}

const (
	// Success describes successfully submitted tx
	Success       = "Success"
	ErrorOnSubmit = "ErrorOnSubmit"
	Committed     = "Committed"
	ErrorOnCommit = "ErrorOnCommit"
)

// Storage is local storage we use to store queries history: known queries, know transactions and its statuses
type Storage interface {
	GetAllPendingTxs() ([]*SubmittedTxInfo, error)
	GetLastQueryHeight(queryID uint64) (block uint64, exists bool, err error)
	SetLastQueryHeight(queryID uint64, block uint64) error
	SetTxStatus(queryID uint64, hash string, neutronHash string, status string) (err error)
	TxExists(queryID uint64, hash string) (exists bool, err error)
	Close() error
}

type SubmittedTxInfo struct {
	QueryID         uint64
	SubmittedTxHash string
	NeutronHash     string
	SubmitTime      time.Time
}
