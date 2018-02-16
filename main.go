package main

import (
	"github.com/SocialNetworkNews/SocialNetworkNews_API/twitter"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	web_api "github.com/SocialNetworkNews/SocialNetworkNews_API/api"
	"log"
	"github.com/rs/cors"
	"github.com/SocialNetworkNews/SocialNetworkNews_API/config"
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
	p := r.PathPrefix("/paper/{uuid}").Subrouter()
	p.HandleFunc("/yesterday", web_api.Yesterday).Methods("GET")
	// cors.Default() setup the middleware with default options being
	// all origins accepted with simple methods (GET, POST). See
	// documentation below for more options.
	handler := cors.Default().Handler(r)

	go func() {
		log.Fatal(http.ListenAndServe(":8000", handler))
	}()

	err := api.StartListening(configData.Lists,configData.Hashtags)
	if err != nil {
		fmt.Println(err)
	}
}
