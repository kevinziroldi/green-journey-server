package mockservers

import (
	"log"
	"net/http"
	"strconv"
)

var tollCostPerKm = 0.09

func StartTollApiServer() {
	http.HandleFunc("/tollapi", TollApiHandler)

	log.Println("Toll API server starting on port 8081")

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

	// extract distance
	distanceString := r.URL.Query().Get("distance")
	if distanceString == "" {
		log.Println("Missing distance value")
		http.Error(w, "Missing distance value", http.StatusBadRequest)
		return
	}
	// convert distance
	distance, err := strconv.Atoi(distanceString)
	if err != nil {
		log.Println("Invalid distance value")
		http.Error(w, "Invalid distance value", http.StatusBadRequest)
		return
	}

	// compute toll cost
	tollCost := tollCostPerKm * float64(distance)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(`{"toll-cost": ` + strconv.FormatFloat(tollCost, 'f', 2, 64) + `}`))
	if err != nil {
		log.Println(err)
		http.Error(w, "error while writing the response", http.StatusInternalServerError)
	}
}
