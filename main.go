package main

import (
	"fmt"
	web_api "github.com/SocialNetworkNews/SocialNetworkNews_API/api"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/config"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/twitter"
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

	r := mux.NewRouter()
	r.HandleFunc("/papers", web_api.Papers).Methods("GET", "POST")

	p := r.PathPrefix("/paper/{uuid}").Subrouter()
	r.HandleFunc("/", web_api.PaperFunc).Methods("GET")
	p.HandleFunc("/yesterday", web_api.Yesterday).Methods("GET")
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
