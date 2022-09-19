package storage

import (
	"encoding/json"
	"fmt"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/syndtr/goleveldb/leveldb/util"
	"strconv"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

const SubmittedTxStatusPrefix = "submitted_txs"

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

func (s *LevelDBStorage) GetAllPendingTxs() ([]*relay.PendingSubmittedTxInfo, error) {
	iterator := s.db.NewIterator(util.BytesPrefix([]byte(SubmittedTxStatusPrefix)), nil)
	defer iterator.Release()
	var txs []*relay.PendingSubmittedTxInfo
	for iterator.Next() {
		value := iterator.Value()
		var txInfo relay.PendingSubmittedTxInfo
		err := json.Unmarshal(value, &txInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into PendingSubmittedTxInfo: %w", err)
		}

		txs = append(txs, &txInfo)
	}
	return txs, nil
}

// GetLastQueryHeight returns last update block for KV query
func (s *LevelDBStorage) GetLastQueryHeight(queryID uint64) (block uint64, err error) {
	s.Lock()
	defer s.Unlock()

	data, err := s.db.Get(uintToBytes(queryID), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("failed getting data from db: %w", err)
	}

	res, err := bytesToUint(data)
	if err != nil {
		return 0, fmt.Errorf("failed converting bytest to uint: %w", err)
	}

	return res, nil
}

// SetTxStatus sets status for given tx
// queryID + hash can be one of 4 statuses:
// 1) Error while submitting tx - relay.ErrorOnSubmit
// 2) tx submitted successfully (temporary status, should be updated after neutron tx committed into the block) - relay.Submitted
//	2.a) failed to commit tx into the block - relay.ErrorOnCommit
//  2.b) tx successfully committed - relay.Committed
// To convert status from "2" to either "2.a" or "2.b" we use additional SubmittedTxStatusPrefix storage to track txs
func (s *LevelDBStorage) SetTxStatus(queryID uint64, hash string, neutronHash string, status relay.SubmittedTxInfo) (err error) {
	s.Lock()
	defer s.Unlock()

	// save tx status
	t, err := s.db.OpenTransaction()
	if err != nil {
		return fmt.Errorf("failed to open leveldb transaction: %w", err)
	}

	defer t.Discard()
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to Marshal SubmittedTxInfo: %w", err)
	}

	err = t.Put(constructKey(queryID, hash), data, nil)
	if err != nil {
		return fmt.Errorf("failed to set tx status: %w", err)
	}

	if status.Status == relay.Submitted {
		txInfo := relay.PendingSubmittedTxInfo{
			QueryID:         queryID,
			SubmittedTxHash: hash,
			NeutronHash:     neutronHash,
			SubmitTime:      time.Now(),
		}
		err = saveIntoPendingQueue(t, neutronHash, txInfo)
		if err != nil {
			return err
		}
	} else if status.Status == relay.Committed || status.Status == relay.ErrorOnCommit {
		err = removeFromPendingQueue(t, neutronHash)
		if err != nil {
			return err
		}
	}

	err = t.Commit()
	return err
}

// TxExists returns if tx has been processed
func (s *LevelDBStorage) TxExists(queryID uint64, hash string) (exists bool, err error) {
	s.Lock()
	defer s.Unlock()

	exists, err = s.db.Has(constructKey(queryID, hash), nil)
	if err != nil {
		return false, fmt.Errorf("failed to get if storage has key: %w", err)
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
	err := s.db.Close()
	if err != nil {
		return fmt.Errorf("failed to close db: %w", err)
	}
	return nil
}

func saveIntoPendingQueue(t *leveldb.Transaction, neutronTXHash string, txInfo relay.PendingSubmittedTxInfo) error {
	key := []byte(SubmittedTxStatusPrefix + neutronTXHash)
	data, err := json.Marshal(txInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal PendingSubmittedTxInfo: %w", err)
	}

	err = t.Put(key, data, nil)
	if err != nil {
		return err
	}

	return nil
}

func removeFromPendingQueue(t *leveldb.Transaction, neutronTXHash string) error {
	key := []byte(SubmittedTxStatusPrefix + neutronTXHash)
	err := t.Delete(key, nil)
	if err != nil {
		return fmt.Errorf("failed to remove PendingSubmittedTxInfo under the key %s: %w", neutronTXHash, err)
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
