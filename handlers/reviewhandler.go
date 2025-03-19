package handlers

import (
	"context"
	"encoding/json"
	"green-journey-server/db"
	"green-journey-server/externals"
	"green-journey-server/model"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func HandleReviews(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getReviewsForCity(w, r)
	case "POST":
		createReview(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func getReviewsForCity(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	cityIata := r.URL.Query().Get("city_iata")
	if cityIata == "" {
		log.Println("Missing city iata")
		http.Error(w, "Missing departure city iata", http.StatusBadRequest)
		return
	}
	countryCode := r.URL.Query().Get("country_code")
	if countryCode == "" {
		log.Println("Missing departure country code")
		http.Error(w, "Missing departure country code", http.StatusBadRequest)
		return
	}
	reviewIDStr := r.URL.Query().Get("review_id")
	if reviewIDStr == "" {
		log.Println("Missing review id")
		http.Error(w, "Missing review id", http.StatusBadRequest)
		return
	}
	reviewID, err := strconv.Atoi(reviewIDStr)
	if err != nil {
		log.Println("Wrong review id")
		http.Error(w, "Wrong review id", http.StatusBadRequest)
		return
	}
	direction, err := strconv.ParseBool(r.URL.Query().Get("direction"))
	if err != nil {
		log.Println("Wrong direction format: ", err)
		http.Error(w, "Wrong direction format", http.StatusBadRequest)
		return
	}

	cityDAO := db.NewCityDAO(db.GetDB())
	city, err := cityDAO.GetCityByIataAndCountryCode(cityIata, countryCode)
	if err != nil {
		log.Println("Error getting city: ", err)
		http.Error(w, "Error getting city", http.StatusBadRequest)
		return
	}

	// get reviews
	reviewDAO := db.NewReviewDAO(db.GetDB())
	var cityReviewElement model.CityReviewElement
	if direction {
		// next reviews
		cityReviewElement, err = reviewDAO.GetNextReviews(city.CityID, reviewID)
	} else {
		// previous reviews
		cityReviewElement, err = reviewDAO.GetPreviousReviews(city.CityID, reviewID)
	}
	if err != nil {
		log.Println("Error getting city review element: ", err)
		http.Error(w, "Error getting city review element", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(cityReviewElement)
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
	firebaseUID, err := externals.VerifyFirebaseToken(ctx, idToken)
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

	// set city id
	if review.CityID == 0 {
		if review.CityIata == "" || review.CountryCode == "" {
			log.Println("Invalid iata and country code")
			http.Error(w, "Invalid iata and country code", http.StatusBadRequest)
			return
		}
		cityDAO := db.NewCityDAO(db.GetDB())
		city, err1 := cityDAO.GetCityByIataAndCountryCode(review.CityIata, review.CountryCode)
		if err1 != nil {
			log.Println("Invalid iata and country code")
			http.Error(w, "Invalid iata and country code", http.StatusBadRequest)
			return
		}
		review.CityID = city.CityID
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
	// reset time zone
	review.DateTime = review.DateTime.UTC()

	// insert review in db
	reviewDAO := db.NewReviewDAO(db.GetDB())
	err = reviewDAO.CreateReview(&review)
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
	firebaseUID, err := externals.VerifyFirebaseToken(ctx, idToken)
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
	// reset time zone
	review.DateTime = review.DateTime.UTC()

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
	firebaseUID, err := externals.VerifyFirebaseToken(ctx, idToken)
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
	review, err := reviewDAO.GetReviewById(reviewID)
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
}

func HandleFirstReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		getFirstReviews(w, r)
	} else {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func getFirstReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	cityIata := r.URL.Query().Get("city_iata")
	if cityIata == "" {
		log.Println("Missing city iata")
		http.Error(w, "Missing departure city iata", http.StatusBadRequest)
		return
	}
	countryCode := r.URL.Query().Get("country_code")
	if countryCode == "" {
		log.Println("Missing departure country code")
		http.Error(w, "Missing departure country code", http.StatusBadRequest)
		return
	}

	cityDAO := db.NewCityDAO(db.GetDB())
	city, err := cityDAO.GetCityByIataAndCountryCode(cityIata, countryCode)
	if err != nil {
		log.Println("Error getting city: ", err)
		http.Error(w, "Error getting city", http.StatusBadRequest)
		return
	}

	reviewDAO := db.NewReviewDAO(db.GetDB())
	cityReviewElement, err := reviewDAO.GetFirstReviewsByCityID(city.CityID)
	if err != nil {
		log.Println("Error getting city review element: ", err)
		http.Error(w, "Error getting city review element", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(cityReviewElement)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
}

func HandleLastReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		getLastReviews(w, r)
	} else {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func getLastReviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	cityIata := r.URL.Query().Get("city_iata")
	if cityIata == "" {
		log.Println("Missing city iata")
		http.Error(w, "Missing departure city iata", http.StatusBadRequest)
		return
	}
	countryCode := r.URL.Query().Get("country_code")
	if countryCode == "" {
		log.Println("Missing departure country code")
		http.Error(w, "Missing departure country code", http.StatusBadRequest)
		return
	}

	cityDAO := db.NewCityDAO(db.GetDB())
	city, err := cityDAO.GetCityByIataAndCountryCode(cityIata, countryCode)
	if err != nil {
		log.Println("Error getting city: ", err)
		http.Error(w, "Error getting city", http.StatusBadRequest)
		return
	}

	reviewDAO := db.NewReviewDAO(db.GetDB())
	cityReviewElement, err := reviewDAO.GetLastReviewsByCityID(city.CityID)
	if err != nil {
		log.Println("Error getting city review element: ", err)
		http.Error(w, "Error getting city review element", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(cityReviewElement)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
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
}
