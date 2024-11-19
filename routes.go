package main

import (
	"fmt"
	"green-journey-server/handlers"
	"log"
	"net/http"
)

func SetupRoutes() {
	// setup routes
	http.HandleFunc("/users", handlers.HandleUsers)
	http.HandleFunc("/users/", handlers.HandleModifyUser)

	http.HandleFunc("/travels/fromto", handlers.HandleTravelsFromTo)
	http.HandleFunc("/travels/user", handlers.HandleTravelsUser)
	http.HandleFunc("/travels/user/", handlers.HandleModifyTravel)

	http.HandleFunc("/reviews", handlers.HandleReviews)
	http.HandleFunc("/reviews/", handlers.HandleModifyReviews)

	http.HandleFunc("/ranking", handlers.HandleRanking)

	fmt.Println("Server starting on port 80")

	// start server
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		// fatal condition
		log.Fatalf("Failed to start the server")
	}
}
