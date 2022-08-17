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

func (s *DummyStorage) GetLastHeight(queryID uint64) (block uint64, err error) {
	return 0, fmt.Errorf("error: can't use dummy storage with non-allowed tx queries")
}

func (s *DummyStorage) SetLastHeight(queryID uint64, block uint64) error {
	s.KVUpdateMap[queryID] = block
	return nil
}

func (s *DummyStorage) GetTxStatusBool(queryID uint64, hash string) (success bool, err error) {
	return false, fmt.Errorf("error: can't use dummy storage with non-allowed tx queries")
}

func (s *DummyStorage) GetTxStatusString(queryID uint64, hash string) (success string, err error) {
	return "", fmt.Errorf("error: can't use dummy storage with non-allowed tx queries")
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

func NewDummyStorage() *DummyStorage {
	s := new(DummyStorage)
	s.KVUpdateMap = make(map[uint64]uint64)
	return s
}

func (s *DummyStorage) GetTx(hash string, block uint64) (exists bool, err error) {
	return false, fmt.Errorf("error: can't use dummy storage with non-allowed tx queries")
}
