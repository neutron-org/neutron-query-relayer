package relay

import (
	"time"
)

const (
	// Success describes successfully submitted tx
	Success       = "Success"
	ErrorOnSubmit = "ErrorOnSubmit"
	Committed     = "Committed"
	ErrorOnCommit = "ErrorOnCommit"
)

// Storage is local storage we use to store queries history: known queries, know transactions and its statuses
type Storage interface {
	GetLastQueryHeight(queryID uint64) (block uint64, exists bool, err error)
	SetLastQueryHeight(queryID uint64, block uint64) error
	SetTxStatus(queryID uint64, hash string, neutronHash string, status string) (err error)
	TxExists(queryID uint64, hash string) (exists bool, err error)
	Close() error
}

type SubmittedTxInfo struct {
	QueryID         uint64
	SubmittedTxHash string
	SubmitTime      time.Time
}
