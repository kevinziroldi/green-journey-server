package handlers

import (
	"green-journey-server/db"
	"log"
	"net/http"
)

func HandleRanking(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		computeRanking(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func computeRanking(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	rankingDAO := db.NewRankingDAO(db.GetDB())
	rankingDAO.ComputeShortDistanceRanking() // return, err
	rankingDAO.ComputeLongDistanceRanking()  // return, err

	// put in json and send response

	// gestire tutto con query SQL, altrimenti devo creare tantissimi oggetti Go

}
