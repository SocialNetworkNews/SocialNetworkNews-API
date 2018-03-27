package twitter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/config"
	"github.com/SocialNetworkNews/anaconda"
	"github.com/dghubble/oauth1"
	"github.com/dghubble/oauth1/twitter"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var apiOnce sync.Once
var api *TwitterAPI

type Tweets struct {
	Tweets []Tweet `json:"tweets,omitempty"`
}

type Tweet struct {
	Username        string   `json:"username"`
	UserID          string   `json:"user_id"`
	DisplayName     string   `json:"display_name"`
	UserProfileLink string   `json:"userprofile_link"`
	TweetLink       string   `json:"tweet_link"`
	Text            string   `json:"text"`
	IMGUrls         []string `json:"image_urls"`
	CreatedAt       string   `json:"created_at"`
	Favorites       string   `json:"favorites"`
	Retweets        string   `json:"retweets"`
	Retweet         bool     `json:"retweet"`
}

type TwitterAPI struct {
	api    *anaconda.TwitterApi
	stream *anaconda.Stream
}

func NewTwitterAPIStruct() *TwitterAPI {
	apiOnce.Do(func() {
		api = &TwitterAPI{}
	})
	return api
}

func (t *TwitterAPI) getTokens(config *oauth1.Config) (string, string, error) {
	requestToken, requestSecret, TokenErr := config.RequestToken()
	if TokenErr != nil {
		return "", "", TokenErr
	}

	authorizationURL, URLErr := config.AuthorizationURL(requestToken)
	if URLErr != nil {
		return "", "", URLErr
	}

	fmt.Println(authorizationURL.String())
	fmt.Print("Enter PinCode: ")
	var pin string
	fmt.Scanln(&pin)

	accessToken, accessSecret, ATErr := config.AccessToken(requestToken, requestSecret, pin)
	if ATErr != nil {
		return "", "", ATErr
	}

	return accessToken, accessSecret, nil

}

func (t *TwitterAPI) Login(consumerKey, consumerSecret string) (err error) {
	oauthConfig := &oauth1.Config{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		Endpoint:       twitter.AuthorizeEndpoint,
	}
	accessToken, accessSecret, ATErr := t.getTokens(oauthConfig)
	if ATErr != nil {
		err = ATErr
		return
	}

	anaconda.SetConsumerKey(consumerKey)
	anaconda.SetConsumerSecret(consumerSecret)
	t.api = anaconda.NewTwitterApi(accessToken, accessSecret)
	return
}

func (t *TwitterAPI) StartListening(lists []string, hashtags []string) error {
	fmt.Println("StartListening")
	Alists, AListErr := t.getLists(lists)
	if AListErr != nil {
		return AListErr
	}
	usersS, MembersErr := t.getListMembersS(Alists)
	if MembersErr != nil {
		return MembersErr
	}

	hastagsS := strings.Join(hashtags, ",")

	v := url.Values{}
	v.Set("follow", usersS)
	v.Set("track", hastagsS)

	t.stream = t.api.PublicStreamFilter(v)
	for tw := range t.stream.C {
		switch v := tw.(type) {
		case anaconda.Tweet:
			currentTime := time.Now().Local()
			t.writeCSV([]string{v.IdStr, currentTime.String()})
			fmt.Printf("%-15s: %s\n", v.User.ScreenName, v.FullText)
		default:
			fmt.Println("Got some other Type: ", v)
		}
	}
	return nil
}

func (t *TwitterAPI) writeCSV(tweet []string) error {
	currentTime := time.Now().Local()
	dataPath := filepath.Join(".", "data")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		os.Mkdir(dataPath, os.ModePerm)
	}
	filename := fmt.Sprintf("tweets_%s.csv", currentTime.Format("2006_01_02"))
	filePath := config.ConfigPath()
	dataFilePath := filepath.Join(filePath, "data", filename)

	// write the file
	f, err := os.OpenFile(dataFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Write(tweet)
	w.Flush()
	defer f.Close()
	return nil
}

func (t *TwitterAPI) getLists(lists []string) ([]anaconda.List, error) {
	var Alists []anaconda.List
	for _, l := range lists {
		s := strings.Split(l, "/")
		v := url.Values{}
		list, err := t.api.GetListBySlug(s[1], s[0], v)
		if err != nil {
			return nil, err
		}
		Alists = append(Alists, list)
	}

	return Alists, nil
}

func (t *TwitterAPI) getListMembersS(lists []anaconda.List) (string, error) {
	var users []anaconda.User
	var usersSS []string
	for _, l := range lists {
		v := url.Values{}
		members, err := t.api.GetListMembers(l.Id, v)
		if err != nil {
			return "", err
		}

		users = append(users, members...)
	}

	for _, u := range users {
		usersSS = append(usersSS, u.IdStr)
	}

	usersS := strings.Join(usersSS, ",")
	return usersS, nil
}

func (t *TwitterAPI) GetTweets(tweets []int64) ([]byte, error) {
	v := url.Values{}
	v.Set("include_entities", "true")
	retweets := make(map[string]bool)
	tweetsSlice := make(map[string]bool)

	var divided [][]int64

	chunkSize := 99

	for i := 0; i < len(tweets); i += chunkSize {
		end := i + chunkSize

		if end > len(tweets) {
			end = len(tweets)
		}

		divided = append(divided, tweets[i:end])
	}

	var JTweets []Tweet

	for _, chunk := range divided {
		tweetsR, tweetErr := t.api.GetTweetsLookupByIds(chunk, v)
		if tweetErr != nil {
			return nil, tweetErr
		}

		for _, t := range tweetsR {
			JT := Tweet{}
			if tweetsSlice[t.IdStr] {
				continue
			}
			if t.RetweetedStatus != nil {
				JT.Retweet = true
				if retweets[t.RetweetedStatus.IdStr] {
					continue
				} else {
					retweets[t.RetweetedStatus.IdStr] = true
				}
				if tweetsSlice[t.RetweetedStatus.IdStr] {
					continue
				}
			} else {
				JT.Retweet = false
			}
			tweetsSlice[t.IdStr] = true
			// If we got a retweet get the data of the original tweet
			if JT.Retweet {
				JT.Username = t.RetweetedStatus.User.ScreenName
				JT.UserID = t.RetweetedStatus.User.IdStr

				JT.DisplayName = t.RetweetedStatus.User.Name
				JT.UserProfileLink = "https://twitter.com/" + t.RetweetedStatus.User.ScreenName
				JT.TweetLink = "https://twitter.com/" + t.RetweetedStatus.User.ScreenName + "/status/" + t.RetweetedStatus.IdStr
				if t.RetweetedStatus.ExtendedTweet.FullText != "" {
					JT.Text = t.RetweetedStatus.ExtendedTweet.FullText
				} else if t.RetweetedStatus.FullText != "" {
					JT.Text = t.RetweetedStatus.FullText
				} else {
					JT.Text = t.RetweetedStatus.Text
				}

				tweetTime, err := t.RetweetedStatus.CreatedAtTime()
				if err != nil {
					return nil, err
				}
				JT.CreatedAt = tweetTime.Format("02.01.2006")
				JT.Favorites = strconv.Itoa(t.RetweetedStatus.FavoriteCount)
				JT.Retweets = strconv.Itoa(t.RetweetedStatus.RetweetCount)

				for _, i := range t.RetweetedStatus.Entities.Media {
					if i.Type == "photo" {
						JT.IMGUrls = append(JT.IMGUrls, i.Media_url_https)
					}
				}
			} else {
				JT.Username = t.User.ScreenName
				JT.UserID = t.User.IdStr

				JT.DisplayName = t.User.Name
				JT.UserProfileLink = "https://twitter.com/" + t.User.ScreenName
				JT.TweetLink = "https://twitter.com/" + t.User.ScreenName + "/status/" + t.IdStr
				if t.ExtendedTweet.FullText != "" {
					JT.Text = t.ExtendedTweet.FullText
				} else if t.FullText != "" {
					JT.Text = t.FullText
				} else {
					JT.Text = t.Text
				}

				tweetTime, err := t.CreatedAtTime()
				if err != nil {
					return nil, err
				}
				JT.CreatedAt = tweetTime.Format("02.01.2006")
				JT.Favorites = strconv.Itoa(t.FavoriteCount)
				JT.Retweets = strconv.Itoa(t.RetweetCount)

				for _, i := range t.Entities.Media {
					if i.Type == "photo" {
						JT.IMGUrls = append(JT.IMGUrls, i.Media_url_https)
					}
				}
			}

			JTweets = append(JTweets, JT)
		}
	}

	tweetsS := Tweets{}
	tweetsS.Tweets = JTweets

	tweetObject, err := json.Marshal(tweetsS)
	if err != nil {
		return nil, err
	}

	return tweetObject, nil
}
