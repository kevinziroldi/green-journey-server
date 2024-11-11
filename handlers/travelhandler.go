package handlers

import (
	"encoding/json"
	"green-journey-server/db"
	"green-journey-server/externals"
	"green-journey-server/model"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TravelOptions struct {
	Options [][]model.Segment `json:"options"`
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
	fromLatitudeStr := r.URL.Query().Get("from_latitude")
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
	fromLongitudeStr := r.URL.Query().Get("from_longitude")
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
	toLatitudeStr := r.URL.Query().Get("to_latitude")
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
	toLongitudeStr := r.URL.Query().Get("to_longitude")
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

	isOutward, err := strconv.ParseBool(r.URL.Query().Get("is_outward"))
	if err != nil {
		log.Println("Wrong isOutward format: ", err)
		http.Error(w, "Wrong isOutward format", http.StatusBadRequest)
		return
	}

	// call all apis and return data
	// always retrieve outward data
	travelOptions := ComputeApiData(from, to, fromLatitude, fromLongitude, toLatitude, toLongitude, departureDate, departureTime, isOutward)

	// build response
	response := TravelOptions{
		Options: travelOptions,
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

	// plane data
	// must be first, because it needs api data
	directionsPlane, err := externals.GetFlights(originName, destinationName, originLatitude, originLongitude, destinationLatitude, destinationLongitude, date, isOutward)
	if err == nil && directionsPlane != nil {
		for i := range directionsPlane {
			if directionsPlane[i] != nil {
				apiData = append(apiData, directionsPlane[i])
			}
		}
	}

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

	// train data
	directionsTrain, err := externals.GetDirectionsTrain(originName, destinationName, originLatitude, originLongitude, destinationLatitude, destinationLongitude, date, t, isOutward)
	if err == nil && directionsTrain != nil {
		apiData = append(apiData, directionsTrain)
	}

	// bus data
	directionsBus, err := externals.GetDirectionsBus(originName, destinationName, originLatitude, originLongitude, destinationLatitude, destinationLongitude, date, t, isOutward)
	if err == nil && directionsBus != nil {
		apiData = append(apiData, directionsBus)
	}

	return apiData
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
	cityDAO := db.NewCityDAO(db.GetDB())
	for _, segment := range travelDetails.Segments {
		if segment.Vehicle != "walk" {
			// check departure city
			departureCity, err1 := cityDAO.GetCityById(segment.DepartureId)
			if err1 != nil || segment.Departure != departureCity.CityName {
				log.Println("Invalid departure city data")
				http.Error(w, "Invalid departure city data", http.StatusBadRequest)
				return
			}
			// check destination city
			destinationCity, err1 := cityDAO.GetCityById(segment.DestinationId)
			if err1 != nil || segment.Destination != destinationCity.CityName {
				log.Println("Invalid destination city data")
				http.Error(w, "Invalid destination city data", http.StatusBadRequest)
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
	// update NumSegment for return segments
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
			// update with final value
			segment.NumSegment = i + 1
		}
	}
	if errorFound {
		log.Println("Invalid num segment")
		http.Error(w, "Invalid num segment", http.StatusBadRequest)
		return
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
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(travelDetails)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
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
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(travel)
	if err != nil {
		log.Println("Error encoding JSON: ", err)
		http.Error(w, "Error encoding", http.StatusInternalServerError)
		return
	}
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
