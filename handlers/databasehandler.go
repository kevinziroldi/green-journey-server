package handlers

import (
	"green-journey-server/db"
	"log"
	"net/http"
)

func HandleResetTestDatabase(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		resetTestDatabase(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func resetTestDatabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	err := db.ResetTestDatabase()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalf("Error resetting test database: %v", err)
	}
}
