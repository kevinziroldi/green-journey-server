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
	http.HandleFunc("travels/user/", handlers.HandleModifyTravel)

	fmt.Println("Server starting on port 8080")

	// start server
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		// fatal condition
		log.Fatalf("Failed to start the server")
	}
}
