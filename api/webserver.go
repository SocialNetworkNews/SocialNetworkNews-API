package api

import (
	"encoding/csv"
	"fmt"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/twitter"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
