package relay

type RelayerStorage interface {
	SetLastUpdateBlock(queryId uint64, block int64) error
	GetLastUpdateBlock(queryID uint64) (block int64, exists bool)
}
