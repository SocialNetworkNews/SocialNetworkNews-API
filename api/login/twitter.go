package login

import (
	"github.com/dghubble/gologin"
	libLogin "github.com/dghubble/gologin/oauth1"
	oauth1Login "github.com/dghubble/gologin/oauth1"
	"github.com/dghubble/gologin/twitter"
	"github.com/dghubble/oauth1"
	"github.com/dghubble/sessions"
	"net/http"
)

var TConfig *oauth1.Config

const (
	sessionName    = "ssn-app"
	sessionSecret  = "example cookie signing secret"
	sessionUserKey = "twitterID"
)

// sessionStore encodes and decodes session data stored in signed cookies
var sessionStore = sessions.NewCookieStore([]byte(sessionSecret), nil)

// issueSession issues a cookie session after successful Twitter login
func IssueSession() http.Handler {
	fn := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		twitterUser, err := twitter.UserFromContext(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// 2. Implement a success handler to issue some form of session
		session := sessionStore.New(sessionName)
		session.Values[sessionUserKey] = twitterUser.ID
		session.Save(w)
		domain := req.Header.Get("Host")
		http.Redirect(w, req, domain, http.StatusFound)
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
		domain := req.Header.Get("Host")
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
