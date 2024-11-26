package handlers

import (
	"context"
	"encoding/json"
	"green-journey-server/db"
	"green-journey-server/model"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func HandleReviews(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getReviewsByCityId(w, r)
	case "POST":
		createReview(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func getReviewsByCityId(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	cityIdStr := r.URL.Query().Get("city_id")
	cityId, err := strconv.Atoi(cityIdStr)
	if err != nil || cityId < 0 {
		log.Println("Wrong id value: ", err)
		http.Error(w, "The provided id is not valid", http.StatusBadRequest)
		return
	}

	reviewDAO := db.NewReviewDAO(db.GetDB())

	reviews, err := reviewDAO.GetReviewsByCity(cityId)
	if err != nil {
		log.Println("Error getting reviews: ", err)
		http.Error(w, "Error getting reviews", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(reviews)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
}

func createReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// get Firebase token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		log.Println("Missing or invalid auth header")
		http.Error(w, "Missing or invalid auth header", http.StatusUnauthorized)
		return
	}
	idToken := strings.TrimPrefix(authHeader, "Bearer ")

	// verify Firebase token
	ctx := context.Background()
	firebaseUID, err := verifyFirebaseToken(ctx, idToken)
	if err != nil {
		log.Println("Unauthorized", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// decode json data
	var review model.Review
	err = json.NewDecoder(r.Body).Decode(&review)
	if err != nil {
		log.Println("Error decoding JSON: ", err)
		http.Error(w, "Invalid data format", http.StatusBadRequest)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			log.Println("Error closing request body:", err)
		}
	}()

	// get user
	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserById(review.UserID)
	if err != nil {
		log.Println("Error getting user: ", err)
		http.Error(w, "Error getting user", http.StatusNotFound)
		return
	}

	// check matching firebaseUID
	if user.FirebaseUID != firebaseUID {
		log.Println("Unauthorized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// check review data
	if review.LocalTransportRating < 1 || review.LocalTransportRating > 5 {
		log.Println("Invalid local transport rating value")
		http.Error(w, "Invalid local transport rating value", http.StatusBadRequest)
		return
	}
	if review.GreenSpacesRating < 1 || review.GreenSpacesRating > 5 {
		log.Println("Invalid green spaces rating value")
		http.Error(w, "Invalid green spaces rating value", http.StatusBadRequest)
		return
	}
	if review.WasteBinsRating < 1 || review.WasteBinsRating > 5 {
		log.Println("Invalid waste bins rating value")
		http.Error(w, "Invalid waste bins rating value", http.StatusBadRequest)
		return
	}

	// insert review in db
	reviewDAO := db.NewReviewDAO(db.GetDB())
	err = reviewDAO.CreateReview(review)
	if err != nil {
		log.Println("Error while interacting with the database: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// send review in response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(review)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func HandleModifyReviews(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		modifyReview(w, r)
	case "DELETE":
		deleteReview(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func modifyReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// get Firebase token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		log.Println("Missing or invalid auth header")
		http.Error(w, "Missing or invalid auth header", http.StatusUnauthorized)
		return
	}
	idToken := strings.TrimPrefix(authHeader, "Bearer ")

	// verify Firebase token
	ctx := context.Background()
	firebaseUID, err := verifyFirebaseToken(ctx, idToken)
	if err != nil {
		log.Println("Unauthorized", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// extract review id from URI
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[2] == "" {
		log.Println("Invalid path")
		http.Error(w, "Review ID not provided", http.StatusBadRequest)
		return
	}
	reviewIDStr := parts[2]
	reviewID, err := strconv.Atoi(reviewIDStr)
	if err != nil || reviewID < 0 {
		log.Println("Invalid user ID")
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// get the review from the body
	var review model.Review
	err = json.NewDecoder(r.Body).Decode(&review)
	if err != nil {
		log.Println("Error while decoding JSON: ", err)
		http.Error(w, "Wrong data provided", http.StatusBadRequest)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			log.Println("Error closing request body:", err)
		}
	}()

	// get user
	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserById(review.UserID)
	if err != nil {
		log.Println("Error getting user: ", err)
		http.Error(w, "Error getting user", http.StatusNotFound)
		return
	}

	// check matching firebaseUID
	if user.FirebaseUID != firebaseUID {
		log.Println("Unauthorized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// check review data
	if review.LocalTransportRating < 1 || review.LocalTransportRating > 5 {
		log.Println("Invalid local transport rating value")
		http.Error(w, "Invalid local transport rating value", http.StatusBadRequest)
		return
	}
	if review.GreenSpacesRating < 1 || review.GreenSpacesRating > 5 {
		log.Println("Invalid green spaces rating value")
		http.Error(w, "Invalid green spaces rating value", http.StatusBadRequest)
		return
	}
	if review.WasteBinsRating < 1 || review.WasteBinsRating > 5 {
		log.Println("Invalid waste bins rating value")
		http.Error(w, "Invalid waste bins rating value", http.StatusBadRequest)
		return
	}

	// update review in db
	reviewDAO := db.NewReviewDAO(db.GetDB())
	err = reviewDAO.UpdateReview(review)
	if err != nil {
		log.Println("Error while interacting with db: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// send review in response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(review)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func deleteReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// get Firebase token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		log.Println("Missing or invalid auth header")
		http.Error(w, "Missing or invalid auth header", http.StatusUnauthorized)
		return
	}
	idToken := strings.TrimPrefix(authHeader, "Bearer ")

	// verify Firebase token
	ctx := context.Background()
	firebaseUID, err := verifyFirebaseToken(ctx, idToken)
	if err != nil {
		log.Println("Unauthorized", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// extract review id from URI
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[2] == "" {
		log.Println("Invalid path")
		http.Error(w, "Review ID not provided", http.StatusBadRequest)
		return
	}
	reviewIDStr := parts[2]
	reviewID, err := strconv.Atoi(reviewIDStr)
	if err != nil || reviewID < 0 {
		log.Println("Invalid user ID")
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// get review
	reviewDAO := db.NewReviewDAO(db.GetDB())
	review, err := reviewDAO.GetReviewsById(reviewID)
	if err != nil {
		log.Println("Review not found: ", err)
		http.Error(w, "Review not found", http.StatusBadRequest)
		return
	}

	// get user
	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserById(review.UserID)
	if err != nil {
		log.Println("Error getting user: ", err)
		http.Error(w, "Error getting user", http.StatusNotFound)
		return
	}

	// check matching firebaseUID
	if user.FirebaseUID != firebaseUID {
		log.Println("Unauthorized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// delete review
	err = reviewDAO.DeleteReview(reviewID)
	if err != nil {
		log.Println("Error while interacting with the db: ", err)
		http.Error(w, "Error while deleting user", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func HandleBestReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		getBestReviews(w, r)
	} else {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func getBestReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// no authentication needed

	// get best reviews
	reviewDAO := db.NewReviewDAO(db.GetDB())
	bestReviews, err := reviewDAO.GetBestReviews()
	if err != nil {
		log.Println("Error getting best reviews: ", err)
		http.Error(w, "Error getting best reviews", http.StatusBadRequest)
		return
	}

	// send response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(bestReviews)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
