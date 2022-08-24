package storage

import "fmt"

type DummyStorage struct {
	KVUpdateMap map[uint64]uint64
}

func (s *DummyStorage) SetTxStatus(queryID uint64, hash string, status string) (err error) {
	return fmt.Errorf("txExists is not yet implemented for DummyStorage")
}

func (s *DummyStorage) SetLastQueryHeight(queryID uint64, block uint64) error {
	s.KVUpdateMap[queryID] = block
	return nil
}

func (s *DummyStorage) TxExists(queryID uint64, hash string) (exists bool, err error) {
	return false, fmt.Errorf("TxExists is not yet implemented for DummyStorages")
}

func (s *DummyStorage) GetLastQueryHeight(queryID uint64) (uint64, bool, error) {
	if val, ok := s.KVUpdateMap[queryID]; ok {
		return val, true, nil
	} else {
		return 0, false, nil
	}
}

func (s *DummyStorage) Close() error {
	return nil
}

func NewDummyStorage() *DummyStorage {
	s := new(DummyStorage)
	s.KVUpdateMap = make(map[uint64]uint64)
	return s
}
