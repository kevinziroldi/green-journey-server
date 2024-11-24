package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
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

// TODO sono da modificare !!!
const travelCoefficient = 10.0
const compensationCoefficient = 10.0
const bonusScore = 100.0

func HandleSearchTravel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		log.Println("Method not supported")
		http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		return
	}

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

	// call all apis and return data
	// always retrieve outward data
	travelOptions := ComputeApiData(departureCity, destinationCity, departureDate, departureTime, isOutward)

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

func ComputeApiData(departureCity, destinationCity model.City, date, t time.Time, isOutward bool) [][]model.Segment {
	var apiData [][]model.Segment

	// plane data
	directionsPlane, err := externals.GetFlights(departureCity, destinationCity, date, isOutward)
	if err == nil && directionsPlane != nil {
		for i := range directionsPlane {
			if directionsPlane[i] != nil {
				apiData = append(apiData, directionsPlane[i])
			}
		}
	}

	// bike data
	directionsBike, err := externals.GetDirectionsBike(departureCity, destinationCity, date, t, isOutward)
	if err == nil && directionsBike != nil {
		apiData = append(apiData, directionsBike)
	}

	// car data
	directionsCar, err := externals.GetDirectionsCar(departureCity, destinationCity, date, t, isOutward)
	if err == nil && directionsCar != nil {
		apiData = append(apiData, directionsCar)
	}

	// train data
	directionsTrain, err := externals.GetDirectionsTrain(departureCity, destinationCity, date, t, isOutward)
	if err == nil && directionsTrain != nil {
		apiData = append(apiData, directionsTrain)
	}

	// bus data
	directionsBus, err := externals.GetDirectionsBus(departureCity, destinationCity, date, t, isOutward)
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

	// if I get an empty list, it is not an error
	// declare empty slice and append, in order to have an empty slice and not nil slice
	travelRequests := []model.TravelDetails{}
	travels, err := travelDAO.GetTravelRequestsByUserId(id)
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

	// update fields in the request

	co2Compensated, formatOk := updateData["co2_compensated"].(float64)
	if formatOk && co2Compensated >= 0 {
		if travel.CO2Compensated < co2Compensated {
			travel.CO2Compensated = co2Compensated
		} else {
			log.Println("CO2 compensated can't decrease: ", err)
			http.Error(w, "CO2 compensated can't decrease", http.StatusBadRequest)
			return
		}
	}

	confirmed, formatOk := updateData["confirmed"].(bool)
	if formatOk {
		if travel.Confirmed == true && confirmed == false {
			log.Println("Travel is already confirmed, change not possible: ", err)
			http.Error(w, "Travel is already confirmed, change not possible", http.StatusBadRequest)
			return
		} else {
			travel.Confirmed = confirmed
		}
	}

	if co2Compensated > 0 && !confirmed {
		log.Println("It is not possible to compensate before confirming")
		http.Error(w, "It is not possible to compensate before confirming", http.StatusBadRequest)
		return
	}

	deltaScore, err := computeDeltaTravelModify(travel, co2Compensated, confirmed)
	if err != nil {
		log.Println("Error computing the score to be added: ", err)
		http.Error(w, "Error computing the score to be added", http.StatusBadRequest)
		return
	}

	// update travel in db
	err = travelDAO.UpdateTravel(travel, deltaScore)
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

func computeDeltaTravelModify(travel model.Travel, co2Compensated float64, confirmed bool) (float64, error) {
	deltaScore := 0.0

	travelDAO := db.NewTravelDAO(db.GetDB())
	travelDetails, err := travelDAO.GetTravelDetailsByTravelID(travel.TravelID)
	if err != nil {
		return 0, err
	}

	// compute total distance and co2 emitted
	totalDistance := 0.0
	totalCO2Emitted := 0.0
	for _, segment := range travelDetails.Segments {
		totalDistance += segment.Distance
		totalCO2Emitted += segment.CO2Emitted
	}

	if !travel.Confirmed && confirmed {
		deltaScore += travelCoefficient * totalDistance / (0.001 + totalCO2Emitted)
	}

	if travel.CO2Compensated < co2Compensated {
		deltaScore += compensationCoefficient * (co2Compensated - travel.CO2Compensated)

		if co2Compensated == totalCO2Emitted {
			deltaScore += bonusScore
		}
	}

	return deltaScore, nil
}

func computeDeltaTravelDelete(travel model.Travel) (float64, error) {
	deltaScore := 0.0

	travelDAO := db.NewTravelDAO(db.GetDB())
	travelDetails, err := travelDAO.GetTravelDetailsByTravelID(travel.TravelID)
	if err != nil {
		return 0, err
	}

	// compute total distance and co2 emitted
	totalDistance := 0.0
	totalCO2Emitted := 0.0
	for _, segment := range travelDetails.Segments {
		totalDistance += segment.Distance
		totalCO2Emitted += segment.CO2Emitted
	}

	deltaScore += travelCoefficient * totalDistance / (0.001 + totalCO2Emitted)
	deltaScore += compensationCoefficient * travel.CO2Compensated
	if travel.CO2Compensated == totalCO2Emitted {
		deltaScore += bonusScore
	}

	return deltaScore, nil
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
	travel, err := travelDAO.GetTravelById(travelID)
	if err != nil {
		log.Println("Invalid travel id")
		http.Error(w, "Invalid travel ID", http.StatusBadRequest)
		return
	}

	deltaScore, err := computeDeltaTravelDelete(travel)
	if err != nil {
		log.Println("Error computing the score to be removed: ", err)
		http.Error(w, "Error computing the score to be removed", http.StatusBadRequest)
		return
	}

	err = travelDAO.DeleteTravel(travelID, deltaScore)
	if err != nil {
		log.Println("Error interacting with the db: ", err)
		http.Error(w, "Error interacting with the db", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
