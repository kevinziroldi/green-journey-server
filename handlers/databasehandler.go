package handlers

import (
	"github.com/joho/godotenv"
	"green-journey-server/db"
	"log"
	"net/http"
	"os"
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

	// retrieve execution mode
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
		http.Error(w, "Error loading .env file", http.StatusInternalServerError)
	}
	testMode := os.Getenv("TEST_MODE")

	switch testMode {
	case "real":
		log.Println("It is not possible to reset the test database in non-test mode")
		http.Error(w, "Error resetting the test database", http.StatusUnauthorized)
	case "test":
		db.ResetTestDatabase()
	default:
		log.Println("Wrong test mode value")
		http.Error(w, "Error resetting the test database", http.StatusUnauthorized)
	}
}
