package storage

import (
	"encoding/json"
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

type TxMap map[string]string

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

func (s *LevelDBStorage) GetTxStatusBool(hash string, block uint64) (success bool, err error) {
	data, err := s.db.Get(uintToBytes(block), nil)
	if err != nil {
		return false, err
	}
	var txmap TxMap

	err = json.Unmarshal(data, &txmap)
	if err != nil {
		return false, err
	}

	if v, ok := txmap[hash]; ok {
		if v == Success {
			return true, nil
		} else {
			return false, nil
		}
	}

	return false, err
}

func (s *LevelDBStorage) SetTxStatus(hash string, block uint64, status string) (err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(uintToBytes(block), nil)

	var txmap TxMap

	err = json.Unmarshal(data, &txmap)
	if err != nil {
		return err
	}

	txmap[hash] = status
	txsBytes, err := json.Marshal(txmap)

	err = s.db.Put(uintToBytes(block), txsBytes, nil)

	if err != nil {
		return err
	}
	return nil
}

func (s *LevelDBStorage) GetTxStatusString(hash string, block uint64) (success string, err error) {
	data, err := s.db.Get(uintToBytes(block), nil)
	if err != nil {
		return "", err
	}

	var txmap TxMap

	err = json.Unmarshal(data, &txmap)
	if err != nil {
		return "", err
	}

	if v, ok := txmap[hash]; ok {
		return v, nil
	}

	return "", err
}

func (s *LevelDBStorage) IsTxExists(hash string, block uint64) (exists bool, err error) {
	data, err := s.db.Get(uintToBytes(block), nil)
	if err != nil {
		return false, err
	}
	var txmap TxMap

	err = json.Unmarshal(data, &txmap)
	if err != nil {
		return false, err
	}

	if _, ok := txmap[hash]; ok {
		return true, nil
	} else {
		return false, nil
	}
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