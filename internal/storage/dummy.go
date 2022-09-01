package storage

import (
	"fmt"
	"github.com/neutron-org/cosmos-query-relayer/internal/relay"
)

type DummyStorage struct {
	KVUpdateMap map[uint64]uint64
}

func (s *DummyStorage) SetTxStatus(queryID uint64, hash string, status string) (err error) {
	return fmt.Errorf("SetTxStatus is not yet implemented for DummyStorage")
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

func (s *DummyStorage) SaveSubmittedTxStatus(neutronTXHash string, txInfo relay.SubmittedTxInfo) error {
	return fmt.Errorf("SaveSubmittedTxStatus is not yet implemented for DummyStorages")
}

func (s *DummyStorage) GetSubmittedTxStatus(neutronTXHash string) (*relay.SubmittedTxInfo, error) {
	return nil, fmt.Errorf("GetSubmittedTxStatus is not yet implemented for DummyStorages")
}

func (s *DummyStorage) RemoveSubmittedTxStatus(neutronTXHash string) error {
	return fmt.Errorf("RemoveSubmittedTxStatus is not yet implemented for DummyStorages")
}

func NewDummyStorage() *DummyStorage {
	s := new(DummyStorage)
	s.KVUpdateMap = make(map[uint64]uint64)
	return s
}
