package mockservers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func StartFuelCostApiServer() {
	http.HandleFunc("/fuelcostapi", FuelCostApiHandler)

	fmt.Println("Fuel cost API server starting on port 8083")

	err := http.ListenAndServe(":8083", nil)
	if err != nil {
		// fatal condition
		log.Fatal("Failed to start Fuel cost API server")
	}
}

func FuelCostApiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	fuelCost := 1.8

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"toll-cost": ` + strconv.FormatFloat(fuelCost, 'f', 2, 64) + `}`))
	if err != nil {
		fmt.Println(err)
		http.Error(w, "error while writing the response", http.StatusInternalServerError)
	}
}
