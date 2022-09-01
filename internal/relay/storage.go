package relay

import (
	"time"
)

// Success describes successfully submitted tx
const Success = "Success"

// Storage is local storage we use to store queries history: known queries, know transactions and its statuses
type Storage interface {
	SaveSubmittedTxStatus(neutronTXHash string, txInfo SubmittedTxInfo) error
	GetSubmittedTxStatus(neutronTXHash string) (*SubmittedTxInfo, error)
	RemoveSubmittedTxStatus(neutronTXHash string) error
	GetLastQueryHeight(queryID uint64) (block uint64, exists bool, err error)
	SetLastQueryHeight(queryID uint64, block uint64) error
	SetTxStatus(queryID uint64, hash string, status string) (err error)
	TxExists(queryID uint64, hash string) (exists bool, err error)
	Close() error
}

type SubmittedTxInfo struct {
	queryID         string
	SubmittedTxHash string
	submitTime      time.Time
}
