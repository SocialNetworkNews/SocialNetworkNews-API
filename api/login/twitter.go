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
	"github.com/dgrijalva/jwt-go"
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

func getUserSecret(id string) (string, error) {
	var secret string
	dataDB, err := db.OpenDB()
	if err != nil {
		return "", err
	}

	DBerr := dataDB.View(func(txn *badger.Txn) error {
		data, err := db.Get(txn, []byte(fmt.Sprintf("users|%s|data", id)))
		secret = fmt.Sprintf("%s", data)
		return err
	})

	return secret, DBerr
}

func saveUser(id, accessToken, accessSecret string, data []byte) error {
	dataDB, err := db.OpenDB()
	if err != nil {
		return err
	}

	return dataDB.Update(func(txn *badger.Txn) error {
		ATerr := txn.Set([]byte(fmt.Sprintf("users|%s|accessToken", id)), []byte(accessToken))
		if ATerr != nil {
			return ATerr
		}

		ASerr := txn.Set([]byte(fmt.Sprintf("users|%s|accessSecret", id)), []byte(accessSecret))
		if ASerr != nil {
			return ASerr
		}

		puuid, UUIDerr := uuid.NewV4()
		if UUIDerr != nil {
			return UUIDerr
		}
		newUUID := puuid.String()

		USerr := txn.Set([]byte(fmt.Sprintf("users|%s|userSecret", id)), []byte(newUUID))
		if USerr != nil {
			return USerr
		}

		return txn.Set([]byte(fmt.Sprintf("users|%s|data", id)), []byte(data))
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
				fmt.Println("error:", err)
			}

			SErr := saveUser(twitterUser.IDStr, accessToken, accessSecret, b)
			if SErr != nil {
				http.Error(w, SErr.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Create a new token object, specifying signing method and the claims
		// you would like it to contain.
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"loggedIn": "true",
		})

		secret, err := getUserSecret(twitterUser.IDStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. Implement a success handler to issue some form of session
		session := sessionStore.New(sessionName)
		session.Values[sessionUserKey] = twitterUser.ID
		session.Save(w)
		w.Header().Set("Authorization", "Bearer "+tokenString)
		w.Header().Set("ID", twitterUser.IDStr)
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
		w.WriteHeader(http.StatusOK)
	}
	w.WriteHeader(http.StatusUnauthorized)
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
