package standalone_storage

import (
	"github.com/Connor1996/badger"
	"github.com/pingcap-incubator/tinykv/kv/config"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
)

// StandAloneStorage is an implementation of `Storage` for a single-node TinyKV instance. It does not
// communicate with other nodes and all data is stored locally.
type StandAloneStorage struct {
	// Your Data Here (1).
	engine *engine_util.Engines
}

func NewStandAloneStorage(conf *config.Config) *StandAloneStorage {
	// Your Code Here (1).
	db_path := conf.DBPath

	kv_path := db_path + "/" + "kv"
	raft_path := db_path + "/" + "raft"

	kv_db := engine_util.CreateDB(kv_path, false)
	raft_db := engine_util.CreateDB(raft_path, true)

	engine := engine_util.NewEngines(kv_db, raft_db, kv_path, raft_path)

	return &StandAloneStorage{
		engine: engine,
	}
}

func (s *StandAloneStorage) Start() error {
	// Your Code Here (1).
	return nil
}

func (s *StandAloneStorage) Stop() error {
	// Your Code Here (1).
	return s.engine.Destroy()
}

func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storage.StorageReader, error) {
	// Your Code Here (1).
	return NewStorageReader(s.engine.Kv.NewTransaction(false)), nil
}

func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error {
	// Your Code Here (1).
	for _, b := range batch {
		switch b.Data.(type) {
		case storage.Put:
			put := b.Data.(storage.Put)
			err := engine_util.PutCF(s.engine.Kv, put.Cf, put.Key, put.Value)
			if err != nil {
				return err
			}
		case storage.Delete:
			del := b.Data.(storage.Delete)
			err := engine_util.DeleteCF(s.engine.Kv, del.Cf, del.Key)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type StorageReader struct {
	kv_txn *badger.Txn
}

func NewStorageReader(txn *badger.Txn) *StorageReader {
	return &StorageReader{
		kv_txn: txn,
	}
}

func (s *StorageReader) GetCF(cf string, key []byte) ([]byte, error) {
	value, err := engine_util.GetCFFromTxn(s.kv_txn, cf, key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return value, err
}

func (s *StorageReader) IterCF(cf string) engine_util.DBIterator {
	return engine_util.NewCFIterator(cf, s.kv_txn)
}

func (s *StorageReader) Close() {
	s.kv_txn.Discard()
}
