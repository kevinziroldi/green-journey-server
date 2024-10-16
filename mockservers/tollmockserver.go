package mockservers

import (
	"fmt"
	"log"
	"net/http"
)

func StartTollApiServer() {
	http.HandleFunc("/tollapi", TollApiHandler)

	fmt.Println("Toll API server starting on port 8081")

	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		// fatal condition
		log.Fatal("Failed to start Toll API server")
	}
}

func TollApiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"toll-cost": 50}`))
	if err != nil {
		fmt.Println(err)
		http.Error(w, "error while writing the response", http.StatusInternalServerError)
	}
}
