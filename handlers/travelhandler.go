package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"green-journey-server/db"
	"green-journey-server/externals"
	"green-journey-server/internals"
	"green-journey-server/model"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TravelOptions struct {
	Options [][]model.Segment `json:"options"`
}

func HandleSearchTravel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()

	// get request parameters

	// departure
	iataDeparture := r.URL.Query().Get("iata_departure")
	if iataDeparture == "" {
		log.Println("Missing departure city iata")
		http.Error(w, "Missing departure city iata", http.StatusBadRequest)
		return
	}
	countryCodeDeparture := r.URL.Query().Get("country_code_departure")
	if countryCodeDeparture == "" {
		log.Println("Missing departure country code")
		http.Error(w, "Missing departure country code", http.StatusBadRequest)
		return
	}

	// destination
	iataDestination := r.URL.Query().Get("iata_destination")
	if iataDestination == "" {
		log.Println("Missing destination city iata")
		http.Error(w, "Missing destination city iata", http.StatusBadRequest)
		return
	}
	countryCodeDestination := r.URL.Query().Get("country_code_destination")
	if countryCodeDestination == "" {
		log.Println("Missing destination country code")
		http.Error(w, "Missing destination country code", http.StatusBadRequest)
		return
	}

	// date
	departureDate, err := time.Parse("2006-01-02", r.URL.Query().Get("date"))
	if err != nil {
		log.Println("Wrong date format: ", err)
		http.Error(w, "Wrong date format", http.StatusBadRequest)
		return
	}
	departureTime, err := time.Parse("15:04", r.URL.Query().Get("time"))
	if err != nil {
		log.Println("Wrong time format: ", err)
		http.Error(w, "Wrong time format", http.StatusBadRequest)
		return
	}

	// outward or not
	isOutward, err := strconv.ParseBool(r.URL.Query().Get("is_outward"))
	if err != nil {
		log.Println("Wrong isOutward format: ", err)
		http.Error(w, "Wrong isOutward format", http.StatusBadRequest)
		return
	}

	// get departure city
	cityDAO := db.NewCityDAO(db.GetDB())
	departureCity, err := cityDAO.GetCityByIataAndCountryCode(iataDeparture, countryCodeDeparture)
	if err != nil || departureCity.CityIata == nil {
		log.Println("Departure city not found: ", err)
		http.Error(w, "Departure city not found", http.StatusBadRequest)
		return
	}
	// get destination city
	destinationCity, err := cityDAO.GetCityByIataAndCountryCode(iataDestination, countryCodeDestination)
	if err != nil || destinationCity.CityIata == nil {
		log.Println("Destination city not found: ", err)
		http.Error(w, "Destination city not found", http.StatusBadRequest)
		return
	}

	// convert date and time to UTC
	departureDate = departureDate.UTC()
	departureTime = departureTime.UTC()

	// call all apis and return data
	// always retrieve outward data
	travelOptions := ComputeApiData(departureCity, destinationCity, departureDate, departureTime, isOutward)

	// build response
	response := TravelOptions{
		Options: travelOptions,
	}

	elapsed := time.Since(start)
	log.Println("TOTAL SEARCH TRAVEL call took:", elapsed)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}
}

func ComputeApiData(departureCity, destinationCity model.City, date, t time.Time, isOutward bool) [][]model.Segment {
	var apiData [][]model.Segment
	var wg sync.WaitGroup
	results := make(chan []model.Segment, 5)

	amadeusCallAPI := func(fetchFunc func(model.City, model.City, time.Time, time.Time, bool) ([][]model.Segment, error)) {
		defer wg.Done()
		directions, err := fetchFunc(departureCity, destinationCity, date, t, isOutward)
		if err == nil && directions != nil {
			for i := range directions {
				if directions[i] != nil {
					results <- directions[i]
				}
			}
		}
	}

	googleMapsCallAPI := func(fetchFunc func(model.City, model.City, time.Time, time.Time, bool) ([]model.Segment, error)) {
		defer wg.Done()
		directions, err := fetchFunc(departureCity, destinationCity, date, t, isOutward)
		if err == nil && directions != nil {
			results <- directions
		}
	}

	wg.Add(1)
	go amadeusCallAPI(func(dep, dest model.City, d, t time.Time, out bool) ([][]model.Segment, error) {
		return externals.GetFlights(dep, dest, d, out)
	})

	wg.Add(1)
	go googleMapsCallAPI(externals.GetDirectionsBike)

	wg.Add(1)
	go googleMapsCallAPI(externals.GetDirectionsCar)

	wg.Add(1)
	go googleMapsCallAPI(externals.GetDirectionsTrain)

	wg.Add(1)
	go googleMapsCallAPI(externals.GetDirectionsBus)

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		apiData = append(apiData, res)
	}

	return apiData
}

func HandleTravelsUser(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getTravelsByUserId(w, r)
	case "POST":
		createTravel(w, r)
	case "PUT":
		modifyTravel(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}
}

func getTravelsByUserId(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
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

	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserByFirebaseUID(firebaseUID)
	if err != nil {
		log.Println("User not found: ", err)
		http.Error(w, "User could not be found", http.StatusNotFound)
		return
	}

	travelDAO := db.NewTravelDAO(db.GetDB())

	// if I get an empty list, it is not an error
	// declare empty slice and append, in order to have an empty slice and not nil slice
	travelRequests := []model.TravelDetails{}
	travels, err := travelDAO.GetTravelRequestsByUserId(user.UserID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("Error getting travels: ", err)
			http.Error(w, "Error getting travels", http.StatusNotFound)
			return
		}
	}
	travelRequests = append(travelRequests, travels...)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(travelRequests)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
}

func createTravel(w http.ResponseWriter, r *http.Request) {
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
	var travelDetails model.TravelDetails
	err = json.NewDecoder(r.Body).Decode(&travelDetails)
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

	// check matching firebaseUID
	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserById(travelDetails.Travel.UserID)
	if err != nil {
		log.Println("User not found", err)
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}
	if user.FirebaseUID != firebaseUID {
		log.Println("Unauthorized", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// check travel data
	// check co2 compensated
	if travelDetails.Travel.CO2Compensated != 0 {
		log.Println("Invalid CO2 values")
		http.Error(w, "Invalid CO2 values", http.StatusBadRequest)
		return
	}

	if travelDetails.Travel.Confirmed == true {
		log.Println("Confirmed must be false")
		http.Error(w, "Confirmed must be false", http.StatusBadRequest)
		return
	}

	// check segments data
	cityDAO := db.NewCityDAO(db.GetDB())
	for _, segment := range travelDetails.Segments {
		if segment.Vehicle != "walk" {
			// check existing departure and destination cities
			_, err1 := cityDAO.GetCityById(segment.DepartureId)
			if err1 != nil {
				log.Println("Invalid departure city id")
				http.Error(w, "Invalid departure city id", http.StatusBadRequest)
				return
			}
			_, err1 = cityDAO.GetCityById(segment.DestinationId)
			if err1 != nil {
				log.Println("Invalid destination city id")
				http.Error(w, "Invalid destination city id", http.StatusBadRequest)
				return
			}
		}
		// check vehicle type
		if segment.Vehicle != "car" &&
			segment.Vehicle != "bike" &&
			segment.Vehicle != "plane" &&
			segment.Vehicle != "train" &&
			segment.Vehicle != "bus" &&
			segment.Vehicle != "walk" {
			log.Println("Invalid data")
			http.Error(w, "Invalid vehicle type", http.StatusBadRequest)
			return
		}
		if segment.Price < 0 {
			log.Println("Invalid data")
			http.Error(w, "Invalid price", http.StatusBadRequest)
			return
		}
		// check co2 values
		if segment.CO2Emitted < 0 {
			log.Println("Invalid data")
			http.Error(w, "Invalid CO2 emitted value", http.StatusBadRequest)
			return
		}
		// check distance
		if segment.Distance < 0 {
			log.Println("Invalid data")
			http.Error(w, "Invalid travel distance", http.StatusBadRequest)
			return
		}
		// check positive num segment
		if segment.NumSegment < 0 {
			log.Println("Invalid data")
			http.Error(w, "Invalid num segment", http.StatusBadRequest)
			return
		}
		// travel id is fake, will be set later
	}

	// check NumSegment (ordered outward segments, followed by ordered return segments)
	numOutwardSegments := -1
	errorFound := false
	for i := 0; i < len(travelDetails.Segments) && !errorFound; i++ {
		segment := travelDetails.Segments[i]

		if segment.IsOutward {
			// check num segment
			if segment.NumSegment != i+1 {
				errorFound = true
			}
		} else {
			// if first return segment, set numOutwardSegments
			if numOutwardSegments == -1 {
				numOutwardSegments = i
			}
			// check num segment
			if segment.NumSegment != i+1-numOutwardSegments {
				errorFound = true
			}
		}
	}
	// update NumSegment for return segments
	for i, _ := range travelDetails.Segments {
		travelDetails.Segments[i].NumSegment = i + 1
	}

	if errorFound {
		log.Println("Invalid num segment")
		http.Error(w, "Invalid num segment", http.StatusBadRequest)
		return
	}

	// reset time zone
	for i, _ := range travelDetails.Segments {
		travelDetails.Segments[i].DateTime = travelDetails.Segments[i].DateTime.UTC()
	}

	// insert travel
	travelDAO := db.NewTravelDAO(db.GetDB())
	travelDetails, err = travelDAO.CreateTravel(travelDetails)
	if err != nil {
		log.Println("Error while interacting with the database: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// send response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(travelDetails)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
}

func modifyTravel(w http.ResponseWriter, r *http.Request) {
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

	// extract travel
	var newTravel model.Travel
	err = json.NewDecoder(r.Body).Decode(&newTravel)
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

	// get travel
	travelDAO := db.NewTravelDAO(db.GetDB())
	existingTravel, err := travelDAO.GetTravelById(newTravel.TravelID)
	if err != nil {
		log.Println("Travel not found: ", err)
		http.Error(w, "Travel not found", http.StatusNotFound)
		return
	}

	// get user
	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserById(newTravel.UserID)
	if err != nil {
		log.Println("User not found: ", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// check matching firebaseUID
	if user.FirebaseUID != firebaseUID {
		log.Println("Unauthorized", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// check provided data
	if existingTravel.Confirmed && !newTravel.Confirmed {
		log.Println("Travel previously confirmed")
		http.Error(w, "Travel already confirmed", http.StatusBadRequest)
		return
	}
	if newTravel.CO2Compensated > 0 && !newTravel.Confirmed {
		log.Println("It is not possible to compensate before confirming")
		http.Error(w, "It is not possible to compensate before confirming", http.StatusBadRequest)
		return
	}
	if newTravel.CO2Compensated < existingTravel.CO2Compensated {
		log.Println("It is not possible to remove compensated CO2")
		http.Error(w, "It is not possible to remove compensated CO2", http.StatusBadRequest)
		return
	}

	travelDetails, err := travelDAO.GetTravelDetailsByTravelID(existingTravel.TravelID)
	if err != nil {
		log.Println("Error retrieving travel details: ", err)
		http.Error(w, "Error retrieving travel details", http.StatusBadRequest)
		return
	}

	deltaScore, isShortDistance, err := internals.ComputeDeltaScoreModify(travelDetails, newTravel.CO2Compensated, newTravel.Confirmed)
	if err != nil {
		log.Println("Error computing the score to be added: ", err)
		http.Error(w, "Error computing the score to be added", http.StatusBadRequest)
		return
	}

	// update travel in db
	err = travelDAO.UpdateTravel(newTravel, deltaScore, isShortDistance)
	if err != nil {
		log.Println("Error interacting with the db: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(newTravel)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
}

func HandleDeleteTravel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		deleteTravel(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
	}
}

// deleting travel from db automatically deletes segments (cascade)
func deleteTravel(w http.ResponseWriter, r *http.Request) {
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

	// extract travel id from URI
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[3] == "" {
		log.Println("Invalid path")
		http.Error(w, "Travel ID not provided", http.StatusBadRequest)
		return
	}
	travelIDStr := parts[3]
	travelID, err := strconv.Atoi(travelIDStr)
	if err != nil || travelID < 0 {
		log.Println("Invalid travel ID")
		http.Error(w, "Invalid travel ID", http.StatusBadRequest)
		return
	}

	// get travel
	travelDAO := db.NewTravelDAO(db.GetDB())
	travel, err := travelDAO.GetTravelById(travelID)
	if err != nil {
		log.Println("Invalid travel id")
		http.Error(w, "Invalid travel ID", http.StatusBadRequest)
		return
	}

	// get user
	userDAO := db.NewUserDAO(db.GetDB())
	user, err := userDAO.GetUserById(travel.UserID)
	if err != nil {
		log.Println("User not found: ", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// check matching firebaseUID
	if user.FirebaseUID != firebaseUID {
		log.Println("Unauthorized", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	travelDetails, err := travelDAO.GetTravelDetailsByTravelID(travel.TravelID)
	if err != nil {
		log.Println("Error retrieving travel details: ", err)
		http.Error(w, "Error retrieving travel details", http.StatusBadRequest)
		return
	}

	deltaScore, isShortDistance, err := internals.ComputeDeltaScoreDelete(travelDetails)
	if err != nil {
		log.Println("Error computing the score to be removed: ", err)
		http.Error(w, "Error computing the score to be removed", http.StatusBadRequest)
		return
	}

	err = travelDAO.DeleteTravel(travelID, deltaScore, isShortDistance)
	if err != nil {
		log.Println("Error interacting with the db: ", err)
		http.Error(w, "Error interacting with the db", http.StatusBadRequest)
		return
	}
}
