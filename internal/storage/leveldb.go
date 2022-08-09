package storage

import (
	"github.com/syndtr/goleveldb/leveldb"
)

type LevelDBStorage struct {
	db *leveldb.DB
}

func (s *LevelDBStorage) NewLevelDBStorage(path string) (*leveldb.DB, error) {
	return leveldb.OpenFile(path, nil)
}

func (s *LevelDBStorage) SetLastUpdateBlock(queryId uint64, block uint64) error {
	return nil
}

func (s *LevelDBStorage) GetLastUpdateBlock(queryID uint64) (block uint64, exists bool) {
	return 0, false
}

func (s *LevelDBStorage) GetTx(hash string, block uint64) (exists bool, err error) {
	return false, nil
}

func (s *LevelDBStorage) SetTxStatus(hash string, block uint64, ok bool) (err error) {
	return nil
}
