package storage

import "fmt"

type DummyStorage struct {
	KVUpdateMap map[uint64]uint64
}

func (s *DummyStorage) GetTxStatusBool(hash string, block uint64) (success bool, err error) {
	return false, fmt.Errorf("error: can't use dummy storage with allowed tx queries")
}

func (s *DummyStorage) GetTxStatusString(hash string, block uint64) (success string, err error) {
	return "", fmt.Errorf("error: can't use dummy storage with allowed tx queries")
}

func (s *DummyStorage) IsTxExists(hash string, block uint64) (exists bool, err error) {
	return false, fmt.Errorf("error: can't use dummy storage with allowed tx queries")
}

func (s *DummyStorage) SetLastUpdateBlock(queryId uint64, block uint64) error {
	s.KVUpdateMap[queryId] = block
	return nil
}

func (s *DummyStorage) GetLastUpdateBlock(queryID uint64) (uint64, bool) {
	if val, ok := s.KVUpdateMap[queryID]; ok {
		return val, true
	} else {
		return 0, false
	}
}

func NewDummyStorage() *DummyStorage {
	s := new(DummyStorage)
	s.KVUpdateMap = make(map[uint64]uint64)
	return s
}

func (s *DummyStorage) GetTx(hash string, block uint64) (exists bool, err error) {
	return false, fmt.Errorf("error: can't use dummy storage with allowed tx queries")
}

func (s *DummyStorage) SetTxStatus(hash string, block uint64, status string) (err error) {
	return fmt.Errorf("error: can't use dummy storage with allowed tx queries")
}
