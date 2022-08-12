package relay

// RelayerStorage is local storage we use to store queries "history", will be nicely implemented via LSC-119
type RelayerStorage interface {
	SetLastUpdateBlock(queryId uint64, block uint64) error
	GetLastUpdateBlock(queryID uint64) (block uint64, exists bool)
	SetTxStatus(queryID uint64, hash string, status string, block uint64) (err error)
	GetTxStatusBool(queryID uint64, hash string) (success bool, err error)
	GetTxStatusString(queryID uint64, hash string) (success string, err error)
	IsTxExists(queryID uint64, hash string) (exists bool, err error)
	IsQueryExists(queryID uint64) (exists bool, err error)
	GetLastHeight(queryID uint64) (block uint64, err error)
	SetLastHeight(queryID uint64, block uint64) error
}
