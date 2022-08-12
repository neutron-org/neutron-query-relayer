package storage

import (
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"strconv"
	"sync"
)

type LevelDBStorage struct {
	sync.Mutex
	db *leveldb.DB
}

const (
	Success string = "success"
)

type QueryMap struct {
	// map [block height]->(map[hash]->status)
	txStatuses map[uint64]map[string]string
	lastHeight uint64
}

func NewLevelDBStorage(path string) (*LevelDBStorage, error) {
	database, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return &LevelDBStorage{db: database}, err
	}

	return &LevelDBStorage{db: database}, nil
}

func (s *LevelDBStorage) SetLastUpdateBlock(queryId uint64, block uint64) error {
	s.Lock()
	defer s.Unlock()

	err := s.db.Put(uintToBytes(queryId), uintToBytes(block), nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *LevelDBStorage) GetLastUpdateBlock(queryID uint64) (block uint64, exists bool) {
	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		return 0, false
	}

	res, err := bytesToUint(data)
	return res, true
}

func (s *LevelDBStorage) GetTxStatusBool(queryID uint64, hash string) (success bool, err error) {
	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		return false, err
	}
	var txmap QueryMap

	err = json.Unmarshal(data, &txmap)
	if err != nil {
		return false, err
	}

	if v, ok := txmap.txStatuses[txmap.lastHeight][hash]; ok {
		if v == Success {
			return true, nil
		} else {
			return false, nil
		}
	}

	return false, err
}

func (s *LevelDBStorage) SetTxStatus(queryID uint64, hash string, status string, block uint64) (err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(uintToBytes(queryID), nil)

	var queryMap QueryMap

	err = json.Unmarshal(data, &queryMap)
	if err != nil {
		return err
	}

	queryMap.txStatuses[block][hash] = status
	txsBytes, err := json.Marshal(queryMap)
	if err != nil {
		return fmt.Errorf("failed to marshall queryMap: %w", err)
	}

	err = s.db.Put(uintToBytes(queryID), txsBytes, nil)

	if err != nil {
		return err
	}
	return nil
}

func (s *LevelDBStorage) GetTxStatusString(queryID uint64, hash string) (success string, err error) {
	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		return "", err
	}

	var txmap QueryMap

	err = json.Unmarshal(data, &txmap)
	if err != nil {
		return "", err
	}

	if v, ok := txmap.txStatuses[txmap.lastHeight][hash]; ok {
		return v, nil
	}

	return "", err
}

func (s *LevelDBStorage) IsTxExists(queryID uint64, hash string) (exists bool, err error) {
	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		return false, err
	}
	var queryMap QueryMap

	err = json.Unmarshal(data, &queryMap)
	if err != nil {
		return false, err
	}

	if _, ok := queryMap.txStatuses[queryMap.lastHeight][hash]; ok {
		return true, nil
	} else {
		return false, nil
	}
}

func (s *LevelDBStorage) GetLastHeight(queryID uint64) (block uint64, err error) {
	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		return 0, err
	}

	var queryMap QueryMap
	err = json.Unmarshal(data, &queryMap)
	if err != nil {
		return 0, err
	}

	return queryMap.lastHeight, nil
}

func (s *LevelDBStorage) SetLastHeight(queryID uint64, block uint64) error {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		return err
	}

	var queryMap QueryMap
	err = json.Unmarshal(data, &queryMap)
	if err != nil {
		return err
	}

	queryMap.lastHeight = block
	txsBytes, err := json.Marshal(queryMap)
	if err != nil {
		return fmt.Errorf("failed to marshall queryMap: %w", err)
	}

	err = s.db.Put(uintToBytes(queryID), txsBytes, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *LevelDBStorage) IsQueryExists(queryID uint64) (exists bool, err error) {
	exists, err = s.db.Has(uintToBytes(queryID), nil)
	if !exists {
		txmap := QueryMap{txStatuses: make(map[uint64]map[string]string), lastHeight: 0}
		txmapBytes, err := json.Marshal(txmap)
		if err != nil {
			return false, fmt.Errorf("failed to marshall txmap: %w", err)
		}

		err = s.db.Put(uintToBytes(queryID), txmapBytes, nil)
	}
	return
}

func uintToBytes(uint642 uint64) []byte {
	return []byte(strconv.FormatUint(uint642, 10))
}

func bytesToUint(bytes []byte) (uint64, error) {
	num, err := strconv.ParseUint(string(bytes), 10, 64)
	if err != nil {
		return 0, err
	}

	return num, nil
}
