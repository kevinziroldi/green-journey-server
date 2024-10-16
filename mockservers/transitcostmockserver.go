package mockservers

import (
	"fmt"
	"log"
	"net/http"
)

func StartTransitCostApiServer() {
	http.HandleFunc("/transitcostapi", TransitCostApiHandler)

	fmt.Println("Transit cost API server starting on port 8082")

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"transit-cost": 100}`))
	if err != nil {
		fmt.Println(err)
		http.Error(w, "error while writing the response", http.StatusInternalServerError)
	}
}
