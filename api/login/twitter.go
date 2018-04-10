package login

import (
	"encoding/json"
	"fmt"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/db"
	"github.com/dghubble/gologin"
	libLogin "github.com/dghubble/gologin/oauth1"
	oauth1Login "github.com/dghubble/gologin/oauth1"
	"github.com/dghubble/gologin/twitter"
	"github.com/dghubble/oauth1"
	"github.com/dghubble/sessions"
	"github.com/dgraph-io/badger"
	"github.com/satori/go.uuid"
	"log"
	"net/http"
)

var TConfig *oauth1.Config

const (
	sessionName    = "ssn-app"
	sessionSecret  = "006b2294-201d-4283-9114-149b3347b264"
	sessionUserKey = "twitterID"
)

// sessionStore encodes and decodes session data stored in signed cookies
var sessionStore = sessions.NewCookieStore([]byte(sessionSecret), nil)

func getUserUUID(id string) (string, error) {
	var uuidS string
	dataDB, err := db.OpenDB()
	if err != nil {
		return "", err
	}

	DBerr := dataDB.View(func(txn *badger.Txn) error {
		data, err := db.Get(txn, []byte(fmt.Sprintf("users|T|%s|uuid", id)))
		uuidS = fmt.Sprintf("%s", data)
		return err
	})

	return uuidS, DBerr
}

func saveTUser(id, accessToken, accessSecret, name string, data []byte) error {
	dataDB, err := db.OpenDB()
	if err != nil {
		return err
	}

	puuid, UUIDerr := uuid.NewV4()
	if UUIDerr != nil {
		return UUIDerr
	}
	newUUID := puuid.String()

	return dataDB.Update(func(txn *badger.Txn) error {
		ATerr := txn.Set([]byte(fmt.Sprintf("users|T|%s|accessToken", id)), []byte(accessToken))
		if ATerr != nil {
			return ATerr
		}

		ASerr := txn.Set([]byte(fmt.Sprintf("users|T|%s|accessSecret", id)), []byte(accessSecret))
		if ASerr != nil {
			return ASerr
		}

		Nerr := txn.Set([]byte(fmt.Sprintf("users|username|T|%s", newUUID)), []byte(name))
		if Nerr != nil {
			return Nerr
		}

		IDerr := txn.Set([]byte(fmt.Sprintf("users|id|T|%s", newUUID)), []byte(id))
		if IDerr != nil {
			return IDerr
		}

		UUIDDBerr := txn.Set([]byte(fmt.Sprintf("users|T|%s|uuid", id)), []byte(newUUID))
		if UUIDDBerr != nil {
			return UUIDDBerr
		}

		return txn.Set([]byte(fmt.Sprintf("users|T|%s|data", id)), []byte(data))
	})
}

func checkUserExists(id string) (bool, error) {
	exists := true
	dataDB, err := db.OpenDB()
	if err != nil {
		return true, err
	}

	dbErr := dataDB.View(func(txn *badger.Txn) error {
		_, QueryErr := txn.Get([]byte(fmt.Sprintf("users|%s", id)))
		if QueryErr != nil && QueryErr != badger.ErrKeyNotFound {
			return QueryErr
		}
		if QueryErr == badger.ErrKeyNotFound {
			exists = false
		}
		return nil
	})
	return exists, dbErr
}

// issueSession issues a cookie session after successful Twitter login
func IssueSession() http.Handler {
	fn := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		twitterUser, err := twitter.UserFromContext(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if exists, err := checkUserExists(twitterUser.IDStr); !exists {
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			accessToken, accessSecret, err := oauth1Login.AccessTokenFromContext(ctx)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			b, err := json.Marshal(twitterUser)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			SErr := saveTUser(twitterUser.IDStr, accessToken, accessSecret, twitterUser.ScreenName, b)
			if SErr != nil {
				http.Error(w, SErr.Error(), http.StatusInternalServerError)
				return
			}
		}

		// 2. Implement a success handler to issue some form of session
		session := sessionStore.New(sessionName)
		session.Values[sessionUserKey] = twitterUser.IDStr
		session.Save(w)

		uuidS, err := getUserUUID(twitterUser.IDStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("UUID", uuidS)
		w.Header().Set("API-VERSION", "0.0.0")
		domain := req.Host
		log.Println("domain:", domain)
		w.WriteHeader(http.StatusOK)
	}
	return http.HandlerFunc(fn)
}

// LogoutHandler destroys the session on POSTs and redirects to home.
func LogoutHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		sessionStore.Destroy(w, sessionName)
	}
	http.Redirect(w, req, "/", http.StatusFound)
}

// IsAuthenticated returns true if the user has a signed session cookie.
func IsAuthenticated(req *http.Request) bool {
	if _, err := sessionStore.Get(req, sessionName); err == nil {
		return true
	}
	return false
}

// IsAuthenticatedHandleFunc returns 200 if the user has a signed session cookie.
func IsAuthenticatedHandleFunc(w http.ResponseWriter, req *http.Request) {
	if IsAuthenticated(req) {
		Reqsession, err := sessionStore.Get(req, sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		twitterUserID := Reqsession.Values[sessionUserKey]
		// We know this is a string
		uuidS, err := getUserUUID(twitterUserID.(string))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("UUID", uuidS)
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

// LoginHandler handles Twitter login requests by obtaining a request token and
// redirecting to the authorization URL.
func LoginHandler(config *oauth1.Config, failure http.Handler) http.Handler {
	// oauth1.LoginHandler -> oauth1.AuthRedirectHandler
	success := AuthRedirectHandler(config, failure)
	return oauth1Login.LoginHandler(config, success, failure)
}

// AuthRedirectHandler reads the request token from the ctx and redirects
// to the authorization URL.
func AuthRedirectHandler(config *oauth1.Config, failure http.Handler) http.Handler {
	if failure == nil {
		failure = gologin.DefaultFailureHandler
	}
	fn := func(w http.ResponseWriter, req *http.Request) {
		domain := req.Host
		log.Println("domain:", domain)
		config.CallbackURL = "https://" + domain + "/login/twitter/callback"
		TConfig = config
		ctx := req.Context()
		requestToken, _, err := libLogin.RequestTokenFromContext(ctx)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		authorizationURL, err := config.AuthorizationURL(requestToken)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		http.Redirect(w, req, authorizationURL.String(), http.StatusFound)
	}
	return http.HandlerFunc(fn)
}
