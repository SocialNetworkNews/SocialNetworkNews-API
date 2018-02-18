package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/db"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/twitter"
	"github.com/dgraph-io/badger"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Yesterday(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	//TODO Use Database or File Structure (probably File Structure)
	uuidVar := vars["uuid"]
	fmt.Println("UUID: ", uuidVar)

	tweets, err := getTweets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("API-VERSION", "0.0.0")
	w.WriteHeader(http.StatusOK)
	w.Write(tweets)
}

type Paper struct {
	Name        string `json:"name,omitempty"`
	UUID        string `json:"uuid,omitempty"`
	Description string `json:"description,omitempty"`
	Author      `json:",omitempty"`
}

type Author struct {
	UUID           string `json:"uuid,omitempty"`
	Username       string `json:"username,omitempty"`
	ProfileIMGURL  string `json:"profile_image_url,omitempty"`
	TwitterProfile string `json:"twitter_profile,omitempty"`
	GoogleProfile  string `json:"google_profile,omitempty"`
	GithubProfile  string `json:"github_profile,omitempty"`
}

func Papers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		papers, err := getPapers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("API-VERSION", "0.0.0")
		w.WriteHeader(http.StatusOK)
		w.Write(papers)
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t []Paper
		err := decoder.Decode(&t)
		if err != nil {
			panic(err)
		}
		defer r.Body.Close()

		papers, err := addPapers(t)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("API-VERSION", "0.0.0")
		w.WriteHeader(http.StatusOK)
		w.Write(papers)
	}
}

func addPapers(data []Paper) ([]byte, error) {
	papersDB, openErr := db.OpenDB()
	if openErr != nil {
		return nil, openErr
	}

	var papers []Paper

	for _, p := range data {
		puuid, UUIDerr := uuid.NewV4()
		if UUIDerr != nil {
			return nil, UUIDerr
		}
		newUUID := puuid.String()
		newName := p.Name
		newDesc := p.Description
		newAuthor := p.Author.UUID

		DBErr := papersDB.Update(func(txn *badger.Txn) error {
			nameErr := txn.Set([]byte(fmt.Sprintf("papers|paper|%s|name", newUUID)), []byte(newName))
			if nameErr != nil {
				return nameErr
			}

			descErr := txn.Set([]byte(fmt.Sprintf("papers|paper|%s|description", newUUID)), []byte(newDesc))
			if descErr != nil {
				return descErr
			}

			return txn.Set([]byte(fmt.Sprintf("papers|paper|%s|author", newUUID)), []byte(newAuthor))
		})
		if DBErr != nil {
			return nil, DBErr
		}

		paper := Paper{}
		paper.UUID = newUUID
		paper.Name = newName

		papers = append(papers, paper)
	}

	papersArray, err := json.Marshal(papers)
	if err != nil {
		return nil, err
	}

	return papersArray, nil
}

func getPapers() ([]byte, error) {
	papersDB, openErr := db.OpenDB()
	if openErr != nil {
		return nil, openErr
	}

	var papers []Paper

	papersDB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		prefix := []byte("papers|paper|")

		known := make(map[string]bool)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			stringKey := fmt.Sprintf("%s", key)
			stringKeySlice := strings.Split(stringKey, "|")
			stringKeyEnd := stringKeySlice[len(stringKeySlice)-1]
			paper := Paper{}
			if known[stringKeyEnd] {
				continue
			}
			known[stringKeyEnd] = true
			paper.UUID = stringKeyEnd

			nameResult, QueryErr := db.Get(txn, []byte(fmt.Sprintf("%s|name", stringKey)))
			if QueryErr != nil {
				return errors.WithMessage(QueryErr, fmt.Sprintf("Key: %s|name", stringKey))
			}

			paper.Name = fmt.Sprintf("%s", nameResult)

			papers = append(papers, paper)
		}
		return nil
	})

	papersArray, err := json.Marshal(papers)
	if err != nil {
		return nil, err
	}

	return papersArray, nil
}

func getTweets() ([]byte, error) {
	api := twitter.NewTwitterAPIStruct()

	// open output file
	currentTime := time.Now().Local()
	currentTime = currentTime.AddDate(0, 0, -1)
	filename := fmt.Sprintf("tweets_%s.csv", currentTime.Format("2006_01_02"))
	dataFilePath := filepath.Join(".", "data", filename)

	fo, err := os.Open(dataFilePath)
	if err != nil {
		return nil, err
	}

	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()

	r := csv.NewReader(fo)
	data, readErr := r.ReadAll()
	if readErr != nil {
		return nil, readErr
	}

	var tweets []int64
	for _, t := range data {
		i, err := strconv.ParseInt(t[0], 10, 64)
		if err != nil {
			return nil, err
		}
		tweets = append(tweets, i)
	}

	tweetObject, err := api.GetTweets(tweets)
	if err != nil {
		return nil, err
	}
	return tweetObject, nil
}
