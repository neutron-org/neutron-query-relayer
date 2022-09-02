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

func (s *LevelDBStorage) GetAllPendingTxs() ([]*relay.SubmittedTxInfo, error) {
	iterator := s.db.NewIterator(util.BytesPrefix([]byte(SubmittedTxStatusPrefix)), nil)
	defer iterator.Release()
	var txs []*relay.SubmittedTxInfo
	for iterator.Next() {
		value := iterator.Value()
		var txInfo relay.SubmittedTxInfo
		err := json.Unmarshal(value, &txInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into SubmittedTxInfo: %w", err)
		}

		txs = append(txs, &txInfo)
	}
	return txs, nil
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
		return 0, false, fmt.Errorf("failed getting data from db: %w", err)
	}

	res, err := bytesToUint(data)
	if err != nil {
		return 0, false, fmt.Errorf("failed converting bytest to uint: %w", err)
	}

	return res, true, nil
}

// SetTxStatus sets status for given tx
// queryID + hash can be one of 4 statuses:
// 1) Error while submitting tx
// 2) tx submitted successfully (temporary status, should be updated after neutron tx committed into the block)
//	2.a) failed to commit tx into the block
//  2.b) tx successfully committed
// To convert status from "2" to either "2.a" or "2.b" we use additional SubmittedTxStatusPrefix storage to track txs
func (s *LevelDBStorage) SetTxStatus(queryID uint64, hash string, neutronHash string, status string) (err error) {
	s.Lock()
	defer s.Unlock()

	// save tx status
	t, err := s.db.OpenTransaction()
	if err != nil {
		return fmt.Errorf("failed to open leveldb tranaction: %w", err)
	}
	defer t.Discard()
	err = t.Put(constructKey(queryID, hash), []byte(status), nil)
	if err != nil {
		return fmt.Errorf("failed to set tx status: %w", err)
	}
	if status == relay.Success {
		txInfo := relay.SubmittedTxInfo{
			QueryID:         queryID,
			SubmittedTxHash: hash,
			NeutronHash:     neutronHash,
			SubmitTime:      time.Now(),
		}
		err = saveIntoPendingQueue(t, neutronHash, txInfo)
		if err != nil {
			return err
		}
	} else if status == relay.Committed || status == relay.ErrorOnCommit {
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

func saveIntoPendingQueue(t *leveldb.Transaction, neutronTXHash string, txInfo relay.SubmittedTxInfo) error {
	key := []byte(SubmittedTxStatusPrefix + neutronTXHash)
	data, err := json.Marshal(txInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal SubmittedTxInfo: %w", err)
	}
	err = t.Put(key, data, nil)
	if err != nil {
		return err
	}
	return nil
}

func getSubmittedTxStatus(t *leveldb.Transaction, neutronTXHash string) (*relay.SubmittedTxInfo, error) {
	key := []byte(SubmittedTxStatusPrefix + neutronTXHash)
	data, err := t.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read SubmittedTxInfo from underlying storage: %w", err)
	}

	var txInfo relay.SubmittedTxInfo
	err = json.Unmarshal(data, &txInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data into SubmittedTxInfo: %w", err)
	}

	return &txInfo, nil
}

func removeFromPendingQueue(t *leveldb.Transaction, neutronTXHash string) error {
	key := []byte(SubmittedTxStatusPrefix + neutronTXHash)
	err := t.Delete(key, nil)
	if err != nil {
		return fmt.Errorf("failed to remove SubmittedTxInfo under the key %s: %w", neutronTXHash, err)
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
