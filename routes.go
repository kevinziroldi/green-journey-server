package main

import (
	"green-journey-server/handlers"
	"net/http"
)

func SetupServer(port string) *http.Server {
	mux := http.NewServeMux()

	// setup routes
	mux.HandleFunc("/users/user", handlers.HandleUsers)
	mux.HandleFunc("/users", handlers.HandleModifyUser)

	mux.HandleFunc("/travels/search", handlers.HandleSearchTravel)
	mux.HandleFunc("/travels/user", handlers.HandleTravelsUser)
	mux.HandleFunc("/travels/user/", handlers.HandleDeleteTravel)

	mux.HandleFunc("/reviews/first", handlers.HandleFirstReviews)
	mux.HandleFunc("/reviews/last", handlers.HandleLastReviews)
	mux.HandleFunc("/reviews/best", handlers.HandleBestReviews)
	mux.HandleFunc("/reviews", handlers.HandleReviews)
	mux.HandleFunc("/reviews/", handlers.HandleModifyReviews)

	mux.HandleFunc("/ranking", handlers.HandleRanking)

	mux.HandleFunc("/resetTestDatabase", handlers.HandleResetTestDatabase)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	return server
}
