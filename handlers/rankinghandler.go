package handlers

import (
	"encoding/json"
	"green-journey-server/db"
	"green-journey-server/model"
	"log"
	"net/http"
	"strconv"
)

type RankingResponse struct {
	ShortDistanceRanking []model.RankingElement `json:"short_distance_ranking"`
	LongDistanceRanking  []model.RankingElement `json:"long_distance_ranking"`
}

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

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 0 {
		log.Println("Wrong id value: ", err)
		http.Error(w, "The provided id is not valid", http.StatusBadRequest)
		return
	}

	userDAO := db.NewUserDAO(db.GetDB())

	_, err = userDAO.GetUserById(id)
	if err != nil {
		log.Println("User not found: ", err)
		http.Error(w, "User could not be found", http.StatusNotFound)
		return
	}

	// compute ranking
	rankingDAO := db.NewRankingDAO(db.GetDB())
	shortDistanceTopUsers, err := rankingDAO.ComputeShortDistanceRanking(id)
	if err != nil {
		log.Println("Error computing ranking: ", err)
		http.Error(w, "Error computing ranking", http.StatusBadRequest)
		return
	}
	longDistanceTopUsers, err := rankingDAO.ComputeLongDistanceRanking(id)
	if err != nil {
		log.Println("Error computing ranking: ", err)
		http.Error(w, "Error computing ranking", http.StatusBadRequest)
		return
	}

	// create response object
	response := RankingResponse{
		ShortDistanceRanking: shortDistanceTopUsers,
		LongDistanceRanking:  longDistanceTopUsers,
	}

	// send response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
