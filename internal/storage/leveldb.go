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

// GetLastUpdateBlock returns last update block for KV query
func (s *LevelDBStorage) GetLastUpdateBlock(queryID uint64) (block uint64, exists bool, err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get last update block: %w", err)
	}

	res, err := bytesToUint(data)
	if err != nil {
		return 0, false, fmt.Errorf("failed to get last update block: %w", err)
	}

	return res, true, nil
}

// SetTxStatus sets status for given tx
func (s *LevelDBStorage) SetTxStatus(queryID uint64, hash string, status string, block uint64) (err error) {
	s.Lock()
	defer s.Unlock()

	// save tx status
	err = s.db.Put(constructKey(queryID, hash), []byte(status), nil)
	if err != nil {
		return fmt.Errorf("failed to set tx status: %w", err)
	}

	// update last processed block
	err = s.db.Put(uintToBytes(queryID), uintToBytes(block), nil)
	if err != nil {
		return fmt.Errorf("failed to set tx status: %w", err)
	}

	return
}

// GetTxStatus returns either "Success" for successfully processed tx or resulting error if tx failed
func (s *LevelDBStorage) GetTxStatus(queryID uint64, hash string) (status string, err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(constructKey(queryID, hash), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get tx status: %w", err)
	}

	return string(data), nil
}

// IsTxExists returns if tx has been processed
func (s *LevelDBStorage) IsTxExists(queryID uint64, hash string) (exists bool, err error) {
	s.Lock()
	defer s.Unlock()

	exists, err = s.db.Has(constructKey(queryID, hash), nil)
	if err != nil {
		return false, fmt.Errorf("failed to check if tx exists: %w", err)
	}

	return exists, nil
}

// SetLastUpdateBlock SetLastHeight sets last processed block to given query
func (s *LevelDBStorage) SetLastUpdateBlock(queryID uint64, block uint64) error {
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
	if err != nil {
		return false, fmt.Errorf("failed check if query with queryID=%d exists in db: %w", queryID, err)
	}
	if !exists {
		err = s.db.Put(uintToBytes(queryID), uintToBytes(0), nil)
		if err != nil {
			return false, fmt.Errorf("failed initialize query w queryID=%d in db with last heigt = 0: %w", queryID, err)
		}
	}
	return
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
