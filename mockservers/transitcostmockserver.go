package mockservers

import (
	"log"
	"net/http"
	"strconv"
)

var trainCostPerKm = 0.11
var busCostPerKm = 0.07

func StartTransitCostApiServer() {
	http.HandleFunc("/transitcostapi", TransitCostApiHandler)

	log.Println("Transit cost API server starting on port 8082")

	err := http.ListenAndServe(":8082", nil)
	if err != nil {
		// fatal condition
		log.Fatal("Failed to start Transit Cost API server")
	}
}

func TransitCostApiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// extract transit mode
	transitMode := r.URL.Query().Get("mode")
	if transitMode == "" {
		log.Println("Missing transit mode value")
		http.Error(w, "Missing transit mode value", http.StatusBadRequest)
		return
	}
	// check transit mode value
	if transitMode != "train" && transitMode != "bus" {
		log.Println("Invalid transit mode value")
		http.Error(w, "Invalid transit mode value", http.StatusBadRequest)
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

	// compute transit cost
	var transitCost float64
	if transitMode == "train" {
		transitCost = trainCostPerKm * float64(distance)
	} else if transitMode == "bus" {
		transitCost = busCostPerKm * float64(distance)
	} else {
		transitCost = 0
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(`{"transit-cost": ` + strconv.FormatFloat(transitCost, 'f', 2, 64) + `}`))
	if err != nil {
		log.Println(err)
		http.Error(w, "error while writing the response", http.StatusInternalServerError)
	}
}
