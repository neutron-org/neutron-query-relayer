package storage

import (
	"fmt"
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

func NewLevelDBStorage(path string) (*LevelDBStorage, error) {
	database, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}

	return &LevelDBStorage{db: database}, nil
}

// GetLastQueryHeight returns last update block for KV query
func (s *LevelDBStorage) GetLastQueryHeight(queryID uint64) (block uint64, exists bool, err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get last query height, error while getting data from db: %w", err)
	}

	res, err := bytesToUint(data)
	if err != nil {
		return 0, false, fmt.Errorf("failed to get last query height, err converting bytest to uint  : %w", err)
	}

	return res, true, nil
}

// SetTxStatus sets status for given tx
func (s *LevelDBStorage) SetTxStatus(queryID uint64, hash string, status string) (err error) {
	s.Lock()
	defer s.Unlock()

	// save tx status
	err = s.db.Put(constructKey(queryID, hash), []byte(status), nil)
	if err != nil {
		return fmt.Errorf("failed to set tx status: %w", err)
	}

	return
}

// TxExists returns if tx has been processed
func (s *LevelDBStorage) TxExists(queryID uint64, hash string) (exists bool, err error) {
	s.Lock()
	defer s.Unlock()

	exists, err = s.db.Has(constructKey(queryID, hash), nil)
	if err != nil {
		return false, fmt.Errorf("failed to check if tx exists: %w", err)
	}

	return exists, nil
}

// SetLastQueryHeight sets last processed block to given query
func (s *LevelDBStorage) SetLastQueryHeight(queryID uint64, block uint64) error {
	s.Lock()
	defer s.Unlock()

	err := s.db.Put(uintToBytes(queryID), uintToBytes(block), nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *LevelDBStorage) Close() error {
	err := s.Close()
	if err != nil {
		return fmt.Errorf("failed to close db: %w", err)
	}
	return nil
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
