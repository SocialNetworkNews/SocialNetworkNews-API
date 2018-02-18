package passwords

import (
	"fmt"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/db"
	"github.com/dgraph-io/badger"
	"golang.org/x/crypto/bcrypt"
)

func SavePassword(password, username string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	userDB, err := db.OpenDB()
	if err != nil {
		return err
	}

	return userDB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(fmt.Sprintf("users|%s|password", username)), hash)
	})
}

func CheckPassword(username, password string) (bool, error) {
	userDB, err := db.OpenDB()
	if err != nil {
		return false, err
	}
	var hash []byte

	DBErr := userDB.View(func(txn *badger.Txn) error {
		var err error
		hash, err = db.Get(txn, []byte(fmt.Sprintf("users|%s|password", username)))
		return err
	})
	if DBErr != nil {
		return false, DBErr
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		return false, err
	}
	return true, nil
}
