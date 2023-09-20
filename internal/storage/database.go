package storage

import (
	"github.com/ava-labs/avalanchego/database"
	avaxLevelDB "github.com/ava-labs/avalanchego/database/leveldb"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func newDatabase(path string) (database.Database, error) {
	db, err := leveldb.OpenFile(path, &opt.Options{
		BlockCacheCapacity:     avaxLevelDB.DefaultBlockCacheSize,
		DisableSeeksCompaction: true,
		OpenFilesCacheCapacity: avaxLevelDB.DefaultHandleCap,
		WriteBuffer:            avaxLevelDB.DefaultWriteBufferSize / 2,
		Filter:                 filter.NewBloomFilter(avaxLevelDB.DefaultBitsPerKey),
		MaxManifestFileSize:    avaxLevelDB.DefaultMaxManifestFileSize,
	})
	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}
	if err != nil {
		return nil, err
	}
	return &avaxLevelDB.Database{
		DB: db,
	}, nil
}
