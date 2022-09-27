package storage

import (
	"fmt"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"
)

type DummyStorage struct {
	KVUpdateMap map[uint64]uint64
}

func (s *DummyStorage) SetTxStatus(queryID uint64, hash string, neutronHash string, status relay.SubmittedTxInfo) (err error) {
	return fmt.Errorf("SetTxStatus is not yet implemented for DummyStorage")
}

func (s *DummyStorage) SetLastQueryHeight(queryID uint64, block uint64) error {
	s.KVUpdateMap[queryID] = block
	return nil
}

func (s *DummyStorage) TxExists(queryID uint64, hash string) (exists bool, err error) {
	return false, fmt.Errorf("TxExists is not yet implemented for DummyStorages")
}

func (s *DummyStorage) GetLastQueryHeight(queryID uint64) (uint64, error) {
	if val, ok := s.KVUpdateMap[queryID]; ok {
		return val, nil
	} else {
		return 0, nil
	}
}

func (s *DummyStorage) GetAllPendingTxs() ([]*relay.PendingSubmittedTxInfo, error) {
	return nil, fmt.Errorf("GetAllPendingTxs is not yet implemented for DummyStorages")
}

func (s *DummyStorage) Close() error {
	return nil
}

func NewDummyStorage() *DummyStorage {
	s := new(DummyStorage)
	s.KVUpdateMap = make(map[uint64]uint64)
	return s
}
