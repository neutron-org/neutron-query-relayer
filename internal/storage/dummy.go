package storage

type DummyStorage struct {
	KVUpdateMap map[uint64]uint64
}

func (s *DummyStorage) SetLastUpdateBlock(queryId uint64, block uint64) error {
	s.KVUpdateMap[queryId] = block
	return nil
}

func (s *DummyStorage) GetLastUpdateBlock(queryID uint64) (uint64, bool) {
	if val, ok := s.KVUpdateMap[queryID]; ok {
		return uint64(val), true
	} else {
		return 0, false
	}
}

func NewDummyStorage() *DummyStorage {
	return new(DummyStorage)
}
