package handlers

import (
	"encoding/json"
	"fmt"
	"green-journey-server/db"
	"green-journey-server/externals"
	"green-journey-server/model"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ApiData struct {
	OutwardOptions [][]model.Segment `json:"outward_options"`
	ReturnOptions  [][]model.Segment `json:"return_options"`
}

func HandleTravelsFromTo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// get request parameters
	from := r.URL.Query().Get("from")
	if from == "" {
		log.Println("Missing departure city value")
		http.Error(w, "Missing departure city value", http.StatusBadRequest)
		return
	}
	to := r.URL.Query().Get("to")
	if to == "" {
		log.Println("Missing arrival city value")
		http.Error(w, "Missing arrival city value", http.StatusBadRequest)
		return
	}
	fromLatitudeStr := r.URL.Query().Get("fromLatitude")
	if fromLatitudeStr == "" {
		log.Println("Missing departure city latitude value")
		http.Error(w, "Missing departure city latitude value", http.StatusBadRequest)
		return
	}
	fromLatitude, err := strconv.ParseFloat(fromLatitudeStr, 64)
	if err != nil {
		log.Println("Invalid departure city latitude value:", err)
		http.Error(w, "Invalid departure city latitude value", http.StatusBadRequest)
		return
	}
	fromLongitudeStr := r.URL.Query().Get("fromLongitude")
	if fromLongitudeStr == "" {
		log.Println("Missing departure city longitude value")
		http.Error(w, "Missing departure city longitude value", http.StatusBadRequest)
		return
	}
	fromLongitude, err := strconv.ParseFloat(fromLongitudeStr, 64)
	if err != nil {
		log.Println("Invalid departure city longitude value:", err)
		http.Error(w, "Invalid departure city longitude value", http.StatusBadRequest)
		return
	}
	toLatitudeStr := r.URL.Query().Get("toLatitude")
	if toLatitudeStr == "" {
		log.Println("Missing arrival city latitude value")
		http.Error(w, "Missing arrival city latitude value", http.StatusBadRequest)
		return
	}
	toLatitude, err := strconv.ParseFloat(toLatitudeStr, 64)
	if err != nil {
		log.Println("Invalid departure city latitude value:", err)
		http.Error(w, "Invalid departure city latitude value", http.StatusBadRequest)
		return
	}
	toLongitudeStr := r.URL.Query().Get("toLongitude")
	if toLongitudeStr == "" {
		log.Println("Missing arrival city longitude value")
		http.Error(w, "Missing arrival city longitude value", http.StatusBadRequest)
		return
	}
	toLongitude, err := strconv.ParseFloat(toLongitudeStr, 64)
	if err != nil {
		log.Println("Invalid arrival city longitude value:", err)
		http.Error(w, "Invalid arrival city longitude value", http.StatusBadRequest)
		return
	}

	dateOutward, err := time.Parse("2006-01-02", r.URL.Query().Get("dateOutward"))
	if err != nil {
		log.Println("Wrong date format: ", err)
		http.Error(w, "Wrong date format", http.StatusBadRequest)
		return
	}
	timeOutward, err := time.Parse("15:04", r.URL.Query().Get("timeOutward"))
	if err != nil {
		log.Println("Wrong time format: ", err)
		http.Error(w, "Wrong time format", http.StatusBadRequest)
		return
	}
	roundTripString := r.URL.Query().Get("round_trip")
	var roundTrip bool
	switch roundTripString {
	case "true":
		roundTrip = true
	case "false":
		roundTrip = false
	default:
		log.Println("Wrong round trip value")
		http.Error(w, "Wrong round trip value", http.StatusBadRequest)
		return
	}
	var dateReturn, timeReturn time.Time
	if roundTrip {
		dateReturn, err = time.Parse("2006-01-02", r.URL.Query().Get("dateReturn"))
		if err != nil {
			log.Println("Wrong date format: ", err)
			http.Error(w, "Wrong date format", http.StatusBadRequest)
			return
		}
		timeReturn, err = time.Parse("15:04", r.URL.Query().Get("timeReturn"))
		if err != nil {
			log.Println("Wrong time format: ", err)
			http.Error(w, "Wrong time format", http.StatusBadRequest)
			return
		}
	}

	// call all apis and return data
	// always retrieve outward data
	outwardData := ComputeApiData(from, to, fromLatitude, fromLongitude, toLatitude, toLongitude, dateOutward, timeOutward, true)
	// check roundTrip to retrieve return data
	var returnData [][]model.Segment
	if roundTrip {
		// invert from and to
		returnData = ComputeApiData(from, to, fromLatitude, fromLongitude, toLatitude, toLongitude, dateReturn, timeReturn, false)
	}

	// build response
	response := ApiData{
		OutwardOptions: outwardData,
		ReturnOptions:  returnData,
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}
}

func ComputeApiData(originName, destinationName string, originLatitude, originLongitude, destinationLatitude, destinationLongitude float64, date, t time.Time, isOutward bool) [][]model.Segment {
	var apiData [][]model.Segment
	// bike data
	directionsBike, err := externals.GetDirectionsBike(originName, destinationName, originLatitude, originLongitude, destinationLatitude, destinationLongitude, date, t, isOutward)
	if err == nil && directionsBike != nil {
		apiData = append(apiData, directionsBike)
	}

	// car data
	directionsCar, err := externals.GetDirectionsCar(originName, destinationName, originLatitude, originLongitude, destinationLatitude, destinationLongitude, date, t, isOutward)
	if err == nil && directionsCar != nil {
		apiData = append(apiData, directionsCar)
	}

	// train data right time
	directionsTrain, err := externals.GetDirectionsTrain(originName, destinationName, originLatitude, originLongitude, destinationLatitude, destinationLongitude, date, t, isOutward)
	if err == nil && directionsTrain != nil {
		apiData = append(apiData, directionsTrain)
	}

	// bus data right time
	directionsBus, err := externals.GetDirectionsBus(originName, destinationName, originLatitude, originLongitude, destinationLatitude, destinationLongitude, date, t, isOutward)
	if err == nil && directionsBus != nil {
		apiData = append(apiData, directionsBus)
	}

	// plane data
	directionsPlane, err := externals.GetFlights(originName, destinationName, originLatitude, originLongitude, destinationLatitude, destinationLongitude, date, isOutward)
	if err == nil && directionsPlane != nil {
		for i := range directionsPlane {
			if directionsPlane[i] != nil {
				apiData = append(apiData, directionsPlane[i])
			}
		}
	}

	return apiData
}

func differentDirections(d1, d2 []model.Segment) bool {
	// return true if they are different, false otherwise

	// compare size
	if len(d1) != len(d2) {
		return true
	}

	// compare segments
	for i := 0; i < len(d1); i++ {
		s1 := d1[i]
		s2 := d2[i]
		if s1.Departure != s2.Departure ||
			s1.Destination != s2.Destination ||
			s1.Date != s2.Date ||
			s1.Hour != s2.Hour ||
			s1.Duration != s2.Duration ||
			s1.Vehicle != s2.Vehicle ||
			s1.Description != s2.Description ||
			s1.Price != s2.Price ||
			s1.CO2Emitted != s2.CO2Emitted ||
			s1.Distance != s2.Distance ||
			s1.NumSegment != s2.NumSegment ||
			s1.IsOutward != s2.IsOutward {
			return true
		}
	}

	return false
}

func HandleTravelsUser(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getTravelsByUserId(w, r)
	case "POST":
		createTravel(w, r)
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

	travelDAO := db.NewTravelDAO(db.GetDB())

	travelRequests, err := travelDAO.GetTravelRequestsByUserId(id)
	if err != nil {
		log.Println("Error getting travels: ", err)
		http.Error(w, "Error getting travels", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(travelRequests)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func createTravel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

	// decode json data
	var travelDetails model.TravelDetails
	err := json.NewDecoder(r.Body).Decode(&travelDetails)
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

	// check travel data
	// check co2 compensated
	if travelDetails.Travel.CO2Compensated < 0 {
		log.Println("Invalid CO2 values")
		http.Error(w, "Invalid CO2 values", http.StatusBadRequest)
		return
	}

	// check segments data
	var segmentNumbers []int
	for _, segment := range travelDetails.Segments {
		// check non-empty strings
		if segment.Departure == "" ||
			segment.Destination == "" ||
			segment.Vehicle == "" {
			log.Println("Missing required fields")
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}
		// check vehicle type
		if segment.Vehicle != "car" &&
			segment.Vehicle != "bike" &&
			segment.Vehicle != "plane" &&
			segment.Vehicle != "train" &&
			segment.Vehicle != "bus" &&
			segment.Vehicle != "walk" {
			fmt.Println("Invalid data")
			http.Error(w, "Invalid vehicle type", http.StatusBadRequest)
			return
		}
		if segment.Price < 0 {
			fmt.Println("Invalid data")
			http.Error(w, "Invalid price", http.StatusBadRequest)
			return
		}
		// check co2 values
		if segment.CO2Emitted < 0 {
			fmt.Println("Invalid data")
			http.Error(w, "Invalid CO2 emitted value", http.StatusBadRequest)
			return
		}
		// check distance
		if segment.Distance < 0 {
			fmt.Println("Invalid data")
			http.Error(w, "Invalid travel distance", http.StatusBadRequest)
			return
		}
		// check num segment
		if segment.NumSegment < 0 {
			fmt.Println("Invalid data")
			http.Error(w, "Invalid num segment", http.StatusBadRequest)
			return
		}
		// append num segment
		segmentNumbers = append(segmentNumbers, segment.NumSegment)
		// check right travel id
		if segment.TravelID != travelDetails.Travel.TravelID {
			fmt.Println("Invalid travel id")
			http.Error(w, "Invalid travel id", http.StatusBadRequest)
			return
		}
	}

	// check num_segments
	sort.Ints(segmentNumbers)
	for i := 0; i < len(segmentNumbers); i++ {
		if segmentNumbers[i] != i+1 {
			fmt.Println("Invalid num segment")
			http.Error(w, "Invalid num segment", http.StatusBadRequest)
			return
		}
	}

	// insert travel
	travelDAO := db.NewTravelDAO(db.GetDB())
	err = travelDAO.CreateTravel(travelDetails)
	if err != nil {
		log.Println("Error while interacting with the database: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// send response
	w.WriteHeader(http.StatusCreated)
}

func HandleModifyTravel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PATCH":
		modifyTravel(w, r)
	case "DELETE":
		deleteTravel(w, r)
	default:
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
	}
}

func modifyTravel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PATCH" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
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

	// get the travel
	travelDAO := db.NewTravelDAO(db.GetDB())
	travel, err := travelDAO.GetTravelById(travelID)
	if err != nil {
		log.Println("Travel not found: ", err)
		http.Error(w, "Travel not found", http.StatusNotFound)
		return
	}

	// get body content
	var updateData map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&updateData)
	if err != nil {
		log.Println("Error decoding JSON: ", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			log.Println("Error closing request body:", err)
		}
	}()

	// update fields in the request (only co2 compensated)
	co2Compensated, isNumber := updateData["co2_compensated"].(float64)
	if isNumber && co2Compensated >= 0 {
		travel.CO2Compensated = co2Compensated
	}

	// update travel in db
	err = travelDAO.UpdateTravel(travel)
	if err != nil {
		log.Println("Error interacting with the db: ", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// deleting travel from db automatically deletes segments (cascade)
func deleteTravel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
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

	travelDAO := db.NewTravelDAO(db.GetDB())
	_, err = travelDAO.GetTravelById(travelID)
	if err != nil {
		log.Println("Invalid travel id")
		http.Error(w, "Invalid travel ID", http.StatusBadRequest)
		return
	}
	err = travelDAO.DeleteTravel(travelID)
	if err != nil {
		log.Println("Error interacting with the db: ", err)
		http.Error(w, "Error interacting with the db", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
