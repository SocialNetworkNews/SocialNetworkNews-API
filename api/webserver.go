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
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Yesterday(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["uuid"]
	fmt.Println("UUID: ", uuid)

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
	Username       string `json:"username,omitempty"`
	ProfileIMGURL  string `json:"profile_image_url,omitempty"`
	TwitterProfile string `json:"twitter_profile,omitempty"`
	GoogleProfile  string `json:"google_profile,omitempty"`
	GithubProfile  string `json:"github_profile,omitempty"`
}

func Papers(w http.ResponseWriter, r *http.Request) {
	papers, err := getPapers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("API-VERSION", "0.0.0")
	w.WriteHeader(http.StatusOK)
	w.Write(papers)
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

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			stringKey := fmt.Sprintf("%s", key)
			stringKeySlice := strings.Split(stringKey, "|")
			stringKeyEnd := stringKeySlice[len(stringKeySlice)-1]
			paper := Paper{}
			paper.UUID = stringKeyEnd

			nameResult, QueryErr := db.Get(txn, []byte(fmt.Sprintf("%s|name", stringKeyEnd)))
			if QueryErr != nil {
				return errors.WithMessage(QueryErr, fmt.Sprintf("Key: %s|name", stringKeyEnd))
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
