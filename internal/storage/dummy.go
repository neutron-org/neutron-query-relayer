package storage

import "fmt"

type DummyStorage struct {
	KVUpdateMap map[uint64]uint64
}

func (s *DummyStorage) SetTxStatus(queryID uint64, hash string, status string, block uint64) (err error) {
	return fmt.Errorf("error: can't use dummy storage with non-allowed tx queries")
}

func (s *DummyStorage) IsQueryExists(queryID uint64) (exists bool, err error) {
	return false, fmt.Errorf("error: can't use dummy storage with non-allowed tx queries")
}

func (s *DummyStorage) SetLastUpdateBlock(queryID uint64, block uint64) error {
	s.KVUpdateMap[queryID] = block
	return nil
}

func (s *DummyStorage) IsTxExists(queryID uint64, hash string) (exists bool, err error) {
	return false, fmt.Errorf("error: can't use dummy storage with non-allowed tx queries")
}

func (s *DummyStorage) GetLastUpdateBlock(queryID uint64) (uint64, bool, error) {
	if val, ok := s.KVUpdateMap[queryID]; ok {
		return val, true, nil
	} else {
		return 0, false, nil
	}
}

func (s *DummyStorage) Close() error {
	return fmt.Errorf("error: can't interact with db using dummy storage")
}

func NewDummyStorage() *DummyStorage {
	s := new(DummyStorage)
	s.KVUpdateMap = make(map[uint64]uint64)
	return s
}
