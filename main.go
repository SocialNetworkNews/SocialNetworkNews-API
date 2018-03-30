package main

import (
	"fmt"
	web_api "github.com/SocialNetworkNews/SocialNetworkNews_API/api"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/api/login"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/config"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/twitter"
	tLogin "github.com/dghubble/gologin/twitter"
	"github.com/dghubble/oauth1"
	twitterOAuth1 "github.com/dghubble/oauth1/twitter"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"log"
	"net/http"
)

func main() {
	api := twitter.NewTwitterAPIStruct()
	configData, confErr := config.GetConfig()
	if confErr != nil {
		log.Fatal(confErr)
	}

	api.Login(configData.ConsumerKey, configData.ConsumerSecret)
	fmt.Println("Logged in!")

	login.TConfig = &oauth1.Config{
		ConsumerKey:    configData.ConsumerKey,
		ConsumerSecret: configData.ConsumerSecret,
		Endpoint:       twitterOAuth1.AuthorizeEndpoint,
	}

	r := mux.NewRouter()
	r.HandleFunc("/papers", web_api.Papers).Methods("GET", "POST")
	r.HandleFunc("/paper/{uuid}", web_api.PaperFunc).Methods("GET")

	p := r.PathPrefix("/paper/{uuid}").Subrouter()
	p.HandleFunc("/yesterday", web_api.Yesterday).Methods("GET")

	lo := r.PathPrefix("/logout").Subrouter()
	lo.HandleFunc("/twitter", login.LogoutHandler)

	li := r.PathPrefix("/login").Subrouter()
	li.Handle("/twitter", login.LoginHandler(login.TConfig, nil))
	li.Handle("/twitter/callback", tLogin.CallbackHandler(login.TConfig, login.IssueSession(), nil))

	// cors.Default() setup the middleware with default options being
	// all origins accepted with simple methods (GET, POST). See
	// documentation below for more options.
	handler := cors.Default().Handler(r)

	go func() {
		log.Fatal(http.ListenAndServe(":8000", handler))
	}()

	err := api.StartListening(configData.Lists, configData.Hashtags)
	if err != nil {
		fmt.Println(err)
	}
}
