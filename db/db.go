package db

import (
	"errors"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/config"
	"github.com/dgraph-io/badger"
	"os"
	"path/filepath"
	"sync"
)

var DB *badger.DB
var onceDB sync.Once

func OpenDB() (db *badger.DB, err error) {
	onceDB.Do(func() {
		// Open the data.db file. It will be created if it doesn't exist.
		filePath := config.ConfigPath()

		if _, StatErr := os.Stat(filepath.Join(filePath, "data")); os.IsNotExist(StatErr) {
			MkdirErr := os.MkdirAll(filepath.Join(filePath, "data"), 0700)
			if MkdirErr != nil {
				err = MkdirErr
				return
			}
		}
		if _, StatErr := os.Stat(filepath.Join(filePath, "data", "cache")); os.IsNotExist(StatErr) {
			MkdirErr := os.MkdirAll(filepath.Join(filePath, "data", "cache"), 0700)
			if MkdirErr != nil {
				err = MkdirErr
				return
			}
		}
		opts := badger.DefaultOptions
		opts.SyncWrites = false
		opts.Dir = filepath.Join(filePath, "data", "cache")
		opts.ValueDir = filepath.Join(filePath, "data", "cache")

		if _, StatErr := os.Stat(filepath.Join(filePath, "data", "cache", "LOCK")); StatErr == nil {
			DeleteErr := os.Remove(filepath.Join(filePath, "data", "cache", "LOCK"))
			if DeleteErr != nil {
				err = DeleteErr
				return
			}
		}

		expDB, DBErr := badger.Open(opts)
		if DBErr != nil {
			err = DBErr
			return
		}
		DB = expDB
	})

	if DB == nil {
		err = errors.New("missing DB")
		return
	}

	db = DB
	return
}

// Get deduplicates all the Gets inside the Database to not repeat that much code.
func Get(txn *badger.Txn, key []byte) (result []byte, err error) {
	item, QueryErr := txn.Get(key)
	if QueryErr != nil && QueryErr != badger.ErrKeyNotFound {
		err = QueryErr
		return
	}
	if QueryErr != badger.ErrKeyNotFound {
		valueByte, valueErr := item.Value()
		result = valueByte
		if valueErr != nil {
			err = valueErr
			return
		}
	}
	return
}
