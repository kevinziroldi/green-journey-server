package main

import (
	"fmt"
	"green-journey-server/handlers"
	"log"
	"net/http"
)

func SetupRoutes(port string) {
	// setup routes
	http.HandleFunc("/users/user", handlers.HandleUsers)
	http.HandleFunc("/users", handlers.HandleModifyUser)

	http.HandleFunc("/travels/search", handlers.HandleSearchTravel)
	http.HandleFunc("/travels/user", handlers.HandleTravelsUser)
	http.HandleFunc("/travels/user/", handlers.HandleDeleteTravel)

	http.HandleFunc("/reviews", handlers.HandleReviews)
	http.HandleFunc("/reviews/", handlers.HandleModifyReviews)
	http.HandleFunc("/reviews/best", handlers.HandleBestReviews)

	http.HandleFunc("/ranking", handlers.HandleRanking)

	fmt.Println("Server starting on port " + port)

	// start server
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		// fatal condition
		log.Fatalf("Failed to start the server")
	}
}
