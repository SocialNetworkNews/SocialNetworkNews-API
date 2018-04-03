package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/config"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/db"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/twitter"
	TLoginStructs "github.com/dghubble/go-twitter/twitter"
	"github.com/dgraph-io/badger"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type TweetsError struct {
	err        string //error description
	statusCode int    // HTTP Code
}

func (e *TweetsError) Error() string {
	return e.err
}

func (e *TweetsError) StatusCode() int {
	return e.statusCode
}

func Yesterday(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// TODO Use Database or File Structure (probably File Structure)
	uuidVar := vars["uuid"]
	fmt.Println("UUID: ", uuidVar)

	tweets, err := getTweets()
	if err != nil {
		if tErr, ok := err.(*TweetsError); ok {
			http.Error(w, tErr.Error(), tErr.StatusCode())
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
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
	PaperImage  string `json:"paper_image,omitempty"`
	Author      `json:"author,omitempty"`
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
	var data []byte
	switch r.Method {
	case "GET":
		papers, err := getPapers(false, "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data = papers
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t []Paper
		err := decoder.Decode(&t)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		papers, err := addPapers(t)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data = papers
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("API-VERSION", "0.0.0")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func PaperFunc(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuidVar := vars["uuid"]

	var data []byte
	switch r.Method {
	case "GET":
		papers, err := getPapers(true, uuidVar)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data = papers
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("API-VERSION", "0.0.0")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func getAuthorData(id string) (*Author, error) {
	author := &Author{}
	dataDB, err := db.OpenDB()
	if err != nil {
		return nil, err
	}

	DBerr := dataDB.View(func(txn *badger.Txn) error {
		username, err := db.Get(txn, []byte(fmt.Sprintf("users|username|T|%s", id)))
		if err != nil {
			return err
		}
		author.Username = fmt.Sprintf("%s", username)

		author.UUID = id

		data, err := db.Get(txn, []byte(fmt.Sprintf("users|T|%s|uuid", id)))
		TUserData := TLoginStructs.User{}
		UMerr := json.Unmarshal(data, &TUserData)
		if UMerr != nil {
			return UMerr
		}
		author.TwitterProfile = TUserData.URL
		author.ProfileIMGURL = TUserData.ProfileImageURLHttps
		return nil
	})

	return author, DBerr
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
		newPIMG := p.PaperImage

		DBErr := papersDB.Update(func(txn *badger.Txn) error {
			nameErr := txn.Set([]byte(fmt.Sprintf("papers|paper|%s|name", newUUID)), []byte(newName))
			if nameErr != nil {
				return nameErr
			}

			descErr := txn.Set([]byte(fmt.Sprintf("papers|paper|%s|description", newUUID)), []byte(newDesc))
			if descErr != nil {
				return descErr
			}

			pIMGErr := txn.Set([]byte(fmt.Sprintf("papers|paper|%s|image", newUUID)), []byte(newPIMG))
			if pIMGErr != nil {
				return pIMGErr
			}

			return txn.Set([]byte(fmt.Sprintf("papers|paper|%s|author", newUUID)), []byte(newAuthor))
		})
		if DBErr != nil {
			return nil, DBErr
		}

		paper := Paper{}
		paper.UUID = newUUID
		paper.Name = newName
		paper.PaperImage = newPIMG
		author, err := getAuthorData(newAuthor)
		if err != nil {
			return nil, err
		}
		paper.Author = *author

		papers = append(papers, paper)
	}

	papersArray, err := json.Marshal(papers)
	if err != nil {
		return nil, err
	}

	return papersArray, nil
}

func getPapers(full bool, uuid string) ([]byte, error) {
	papersDB, openErr := db.OpenDB()
	if openErr != nil {
		return nil, openErr
	}

	var papers []Paper

	papersDB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		var prefix []byte
		if full {
			prefix = []byte(fmt.Sprintf("papers|paper|%s", uuid))
		} else {
			prefix = []byte("papers|paper|")
		}

		known := make(map[string]bool)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			stringKey := fmt.Sprintf("%s", key)
			stringKeySlice := strings.Split(stringKey, "|")
			var stringKeyEnd string
			if full {
				stringKeyEnd = ""
			} else {
				stringKeyEnd = stringKeySlice[len(stringKeySlice)-2]
			}

			paper := Paper{}
			if known[stringKeyEnd] {
				continue
			}
			known[stringKeyEnd] = true
			paper.UUID = stringKeyEnd

			nameResult, QueryErr := db.Get(txn, []byte(fmt.Sprintf("%s%s|name", prefix, stringKeyEnd)))
			if QueryErr != nil {
				return errors.WithMessage(QueryErr, fmt.Sprintf("%s%s|name", prefix, stringKeyEnd))
			}

			paperIMGResult, QueryErr := db.Get(txn, []byte(fmt.Sprintf("%s%s|image", prefix, stringKeyEnd)))
			if QueryErr != nil {
				return errors.WithMessage(QueryErr, fmt.Sprintf("%s%s|image", prefix, stringKeyEnd))
			}

			descResult, QueryErr := db.Get(txn, []byte(fmt.Sprintf("%s%s|description", prefix, stringKeyEnd)))
			if QueryErr != nil {
				return errors.WithMessage(QueryErr, fmt.Sprintf("%s%s|description", prefix, stringKeyEnd))
			}

			paper.PaperImage = fmt.Sprintf("%s", paperIMGResult)
			paper.Name = fmt.Sprintf("%s", nameResult)
			paper.Description = fmt.Sprintf("%s", descResult)

			if full {
				AUUIDResult, QueryErr := db.Get(txn, []byte(fmt.Sprintf("%s%s|author", prefix, stringKeyEnd)))
				if QueryErr != nil {
					return errors.WithMessage(QueryErr, fmt.Sprintf("%s%s|author", prefix, stringKeyEnd))
				}
				authorID := fmt.Sprintf("%s", AUUIDResult)
				log.Println(authorID)
				author, err := getAuthorData(authorID)
				if err != nil {
					return err
				}
				paper.Author = *author
			}
			log.Printf("%+v", paper)
			papers = append(papers, paper)
		}
		return nil
	})

	var papersArray []byte
	var err error
	if full {
		papersArray, err = json.Marshal(papers[0])
		if err != nil {
			return nil, err
		}
	} else {
		papersArray, err = json.Marshal(papers)
		if err != nil {
			return nil, err
		}
	}

	return papersArray, nil
}

func getTweets() ([]byte, error) {
	api := twitter.NewTwitterAPIStruct()

	// open output file
	currentTime := time.Now().Local()
	currentTime = currentTime.AddDate(0, 0, -1)
	filePath := config.ConfigPath()
	filename := fmt.Sprintf("tweets_%s.csv", currentTime.Format("2006_01_02"))
	dataFilePath := filepath.Join(filePath, "data", filename)

	fo, err := os.Open(dataFilePath)
	if err != nil {
		return nil, &TweetsError{err.Error(), 404}
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
