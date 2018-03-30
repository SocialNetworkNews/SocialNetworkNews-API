package login

import (
	"github.com/dghubble/gologin/twitter"
	"github.com/dghubble/sessions"
	"net/http"
)

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
