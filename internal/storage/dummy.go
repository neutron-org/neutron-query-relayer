package storage

type DummyStorage struct {
	KVUpdateMap map[uint64]int64
}

func (s *DummyStorage) SetLastUpdateBlock(queryId uint64, block int64) error {
	s.KVUpdateMap[queryId] = block
	return nil
}

func (s *DummyStorage) GetLastUpdateBlock(queryID uint64) (int64, bool) {
	if val, ok := s.KVUpdateMap[queryID]; ok {
		return val, true
	} else {
		return 0, false
	}
}

func Init() *DummyStorage {
	return new(DummyStorage)
}
