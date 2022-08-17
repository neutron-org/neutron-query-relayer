package storage

import (
	"strconv"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
)

// LevelDBStorage Basically has a simple structure inside: we have 2 maps
// first one : map of queryID -> last block this query has been processed
// second one: map of queryID+txHash -> status of sent tx
type LevelDBStorage struct {
	sync.Mutex
	db *leveldb.DB
}

const (
	Success string = "success"
)

func NewLevelDBStorage(path string) (*LevelDBStorage, error) {
	database, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return &LevelDBStorage{db: database}, err
	}

	return &LevelDBStorage{db: database}, nil
}

// GetLastUpdateBlock returns last update block for KV query
func (s *LevelDBStorage) GetLastUpdateBlock(queryID uint64) (block uint64, exists bool, err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		return 0, false, err
	}

	res, err := bytesToUint(data)
	if err != nil {
		return 0, false, err
	}

	return res, true, err
}

// GetTxStatusBool returns boolean status of processed tx
func (s *LevelDBStorage) GetTxStatusBool(queryID uint64, hash string) (success bool, err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(constructKey(queryID, hash), nil)
	if err != nil {
		return false, err
	}

	if string(data) == Success {
		return true, nil
	} else {
		return false, nil
	}
}

// SetTxStatus sets status for given tx
func (s *LevelDBStorage) SetTxStatus(queryID uint64, hash string, status string, block uint64) (err error) {
	s.Lock()
	defer s.Unlock()

	// save tx status
	err = s.db.Put(constructKey(queryID, hash), []byte(status), nil)
	if err != nil {
		return err
	}

	// update last processed block
	err = s.db.Put(uintToBytes(queryID), uintToBytes(block), nil)
	if err != nil {
		return err
	}

	return
}

// GetTxStatusString returns either "Success" for successfully processed tx or resulting error if tx failed
func (s *LevelDBStorage) GetTxStatusString(queryID uint64, hash string) (status string, err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(constructKey(queryID, hash), nil)
	if err != nil {
		return "", err
	}

	return string(data), err
}

// IsTxExists returns if tx has been processed
func (s *LevelDBStorage) IsTxExists(queryID uint64, hash string) (exists bool, err error) {
	s.Lock()
	defer s.Unlock()

	exists, err = s.db.Has(constructKey(queryID, hash), nil)
	if err != nil {
		return false, err
	}

	return
}

// GetLastHeight returns last processed block to given query
func (s *LevelDBStorage) GetLastHeight(queryID uint64) (block uint64, err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		return 0, err
	}

	val, err := bytesToUint(data)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// SetLastHeight sets last processed block to given query
func (s *LevelDBStorage) SetLastHeight(queryID uint64, block uint64) error {
	s.Lock()
	defer s.Unlock()

	err := s.db.Put(uintToBytes(queryID), uintToBytes(block), nil)
	if err != nil {
		return err
	}

	return nil
}

// IsQueryExists returns if query exists, also if not - last processed block to 0
func (s *LevelDBStorage) IsQueryExists(queryID uint64) (exists bool, err error) {
	s.Lock()
	defer s.Unlock()

	exists, err = s.db.Has(uintToBytes(queryID), nil)
	if !exists {
		err = s.db.Put(uintToBytes(queryID), uintToBytes(0), nil)
	}
	return
}

func uintToBytes(num uint64) []byte {
	return []byte(strconv.FormatUint(num, 10))
}

func bytesToUint(bytes []byte) (uint64, error) {
	num, err := strconv.ParseUint(string(bytes), 10, 64)
	if err != nil {
		return 0, err
	}

	return num, nil
}

func constructKey(num uint64, str string) []byte {
	return append(uintToBytes(num), str...)
}
