package storage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	SubmittedTxStatusPrefix    = "submitted_txs"
	UnsuccessfulTxStatusPrefix = "unsuccessful_txs"
	CachedTxs                  = "cached_txs"
)

// LevelDBStorage Basically has a simple structure inside: we have 2 maps
// first one : map of queryID -> last block this query has been processed
// second one: map of queryID+txHash -> status of sent tx
type LevelDBStorage struct {
	mutex sync.Mutex
	db    *leveldb.DB
}

func NewLevelDBStorage(path string) (*LevelDBStorage, error) {
	database, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize new stirage: %w", err)
	}

	return &LevelDBStorage{db: database}, nil
}

func (s *LevelDBStorage) GetAllPendingTxs() ([]*relay.PendingSubmittedTxInfo, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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

func (s *LevelDBStorage) GetAllUnsuccessfulTxs() ([]*relay.UnsuccessfulTxInfo, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	iterator := s.db.NewIterator(util.BytesPrefix([]byte(UnsuccessfulTxStatusPrefix)), nil)
	defer iterator.Release()
	// use `make` to avoid printing empty value in json as `null`
	var txs = make([]*relay.UnsuccessfulTxInfo, 0)
	for iterator.Next() {
		value := iterator.Value()
		var txInfo relay.UnsuccessfulTxInfo
		err := json.Unmarshal(value, &txInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into UnsuccessfulTxInfo: %w", err)
		}

		txs = append(txs, &txInfo)
	}
	return txs, nil
}

// GetCachedTx returns a cached remote tx
func (s *LevelDBStorage) GetCachedTx(queryID uint64, hash string) (*relay.Transaction, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := constructCacheTxKey(queryID, hash)
	data, err := s.db.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Transaction for query_id + hash {%d %s}: %w", queryID, hash, err)
	}

	fmt.Println("DATA: ", data)
	fmt.Println("DATA STRING: ", string(data))

	var txInfo relay.Transaction
	err = json.Unmarshal(data, &txInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data into Transaction: %w", err)
	}
	fmt.Println("DECODED TX: ", txInfo)

	txMap := make(map[string]interface{})
	err = json.Unmarshal(data, &txMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data into Transaction: %w", err)
	}
	fmt.Println("DECODED TXMAP: ", txMap)

	return &txInfo, nil
}

// GetLastQueryHeight returns last update block for KV query
func (s *LevelDBStorage) GetLastQueryHeight(queryID uint64) (block uint64, found bool, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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

func (s *LevelDBStorage) SaveTxData(queryID uint64, hash string, tx relay.Transaction) error {
	return nil
}

func (s *LevelDBStorage) RemoveTxData(queryID uint64, hash string) error {
	return nil
}

// SetTxStatus sets status for given tx
// queryID + hash can be one of 4 statuses:
//  1. Error while submitting tx - relay.ErrorOnSubmit
//  2. tx submitted successfully (temporary status, should be updated after neutron tx committed into the block) - relay.Submitted
//     2.a) failed to commit tx into the block - relay.ErrorOnCommit
//     2.b) tx successfully committed - relay.Committed
//
// To convert status from "2" to either "2.a" or "2.b" we use additional SubmittedTxStatusPrefix storage to track txs
func (s *LevelDBStorage) SetTxStatus(queryID uint64, hash string, neutronHash string, txInfo relay.SubmittedTxInfo, processedTx *relay.Transaction) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	t, err := s.db.OpenTransaction()
	if err != nil {
		return fmt.Errorf("failed to open leveldb transaction: %w", err)
	}

	defer t.Discard()
	data, err := json.Marshal(txInfo)
	if err != nil {
		return fmt.Errorf("failed to Marshal SubmittedTxInfo: %w", err)
	}

	err = t.Put(constructTxStatusKey(queryID, hash), data, nil)
	if err != nil {
		return fmt.Errorf("failed to set tx txInfo: %w", err)
	}

	if processedTx != nil {
		err = cacheProcessedTx(t, queryID, hash, *processedTx)
		if err != nil {
			return fmt.Errorf("failed to cache processed tx: %w", err)
		}
	}

	if txInfo.Status == relay.Submitted {
		pendingTxInfo := relay.PendingSubmittedTxInfo{
			QueryID:         queryID,
			SubmittedTxHash: hash,
			NeutronHash:     neutronHash,
		}
		err = saveIntoPendingQueue(t, neutronHash, pendingTxInfo)
		if err != nil {
			return fmt.Errorf("failed to saved txInfo into pending queue: %w", err)
		}
	} else if txInfo.Status == relay.Committed || txInfo.Status == relay.ErrorOnCommit {
		err = removeFromPendingQueue(t, neutronHash)
		if err != nil {
			return fmt.Errorf("failed to remove txInfo from pending queue: %w", err)
		}
	}

	if txInfo.Status == relay.ErrorOnCommit || txInfo.Status == relay.ErrorOnSubmit {
		unsuccessfulTxInfo := relay.UnsuccessfulTxInfo{
			QueryID:         queryID,
			SubmittedTxHash: hash,
			NeutronHash:     neutronHash,
			ErrorTime:       time.Now(),
			Status:          txInfo.Status,
			Message:         txInfo.Message,
		}
		err = saveIntoUnsuccessfulQueue(t, queryID, hash, unsuccessfulTxInfo)
		if err != nil {
			return err
		}
	}

	if txInfo.Status == relay.Committed {
		err = removeFromUnsuccessfulQueue(t, queryID, hash)
		if err != nil {
			return err
		}

		err = removeCachedTx(t, queryID, hash)
		if err != nil {
			return fmt.Errorf("failed to saved txInfo into error queue: %w", err)
		}
	}

	err = t.Commit()
	return err
}

// TxExists returns if tx has been processed
func (s *LevelDBStorage) TxExists(queryID uint64, hash string) (exists bool, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	exists, err = s.db.Has(constructTxStatusKey(queryID, hash), nil)
	if err != nil {
		return false, fmt.Errorf("failed to get if storage has key: %w", err)
	}

	return exists, nil
}

// SetLastQueryHeight sets last processed block to given query
func (s *LevelDBStorage) SetLastQueryHeight(queryID uint64, block uint64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
	key := constructPendingQueueKey(neutronTXHash)
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
	key := constructPendingQueueKey(neutronTXHash)
	err := t.Delete(key, nil)
	if err != nil {
		return fmt.Errorf("failed to remove PendingSubmittedTxInfo under the key %s: %w", neutronTXHash, err)
	}

	return nil
}

func saveIntoUnsuccessfulQueue(t *leveldb.Transaction, queryID uint64, tXHash string, txInfo relay.UnsuccessfulTxInfo) error {
	key := constructUnsuccessfulQueueKey(queryID, tXHash)
	data, err := json.Marshal(txInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal UnsuccessfulTxInfo: %w", err)
	}

	err = t.Put(key, data, nil)
	if err != nil {
		return err
	}

	return nil
}

func removeFromUnsuccessfulQueue(t *leveldb.Transaction, queryID uint64, tXHash string) error {
	key := constructUnsuccessfulQueueKey(queryID, tXHash)
	err := t.Delete(key, nil)
	if err != nil {
		return fmt.Errorf("failed to remove UnsuccessfulTxInfo under the key %d%s: %w", queryID, tXHash, err)
	}

	return nil
}

func removeCachedTx(t *leveldb.Transaction, queryID uint64, tXHash string) error {
	key := constructCacheTxKey(queryID, tXHash)
	err := t.Delete(key, nil)
	if err != nil {
		return fmt.Errorf("failed to remove cached tx under the key %d%s: %w", queryID, tXHash, err)
	}

	return nil
}

func cacheProcessedTx(t *leveldb.Transaction, queryID uint64, tXHash string, tx relay.Transaction) error {
	key := constructCacheTxKey(queryID, tXHash)
	data, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal relay.Transaction: %w", err)
	}

	fmt.Println(tx)

	err = t.Put(key, data, nil)
	if err != nil {
		return err
	}

	return nil
}

func constructCacheTxKey(queryID uint64, tXHash string) []byte {
	return append([]byte(CachedTxs), constructTxStatusKey(queryID, tXHash)...)
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

func constructUnsuccessfulQueueKey(queryID uint64, neutronTXHash string) []byte {
	key := append([]byte(UnsuccessfulTxStatusPrefix), constructTxStatusKey(queryID, neutronTXHash)...)
	return key
}

func constructPendingQueueKey(neutronTXHash string) []byte {
	key := []byte(SubmittedTxStatusPrefix + neutronTXHash)
	return key
}

func constructTxStatusKey(num uint64, str string) []byte {
	return append(uintToBytes(num), str...)
}
