package relay

// Success describes successfully submitted tx
const Success = "Success"

// Storage is local storage we use to store queries "history", will be nicely implemented via LSC-119
type Storage interface {
	GetLastUpdateBlock(queryID uint64) (block uint64, exists bool, err error)
	SetLastUpdateBlock(queryID uint64, block uint64) error
	SetTxStatus(queryID uint64, hash string, status string, block uint64) (err error)
	IsTxExists(queryID uint64, hash string) (exists bool, err error)
	Close() error
}
