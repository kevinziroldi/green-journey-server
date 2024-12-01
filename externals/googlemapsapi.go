package externals

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"green-journey-server/db"
	"green-journey-server/internals"
	"green-journey-server/model"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

var googleApiKey string

// car directions

type DistanceMatrixResponse struct {
	DestinationAddresses []string `json:"destination_addresses"`
	OriginAddresses      []string `json:"origin_addresses"`
	Rows                 []Row    `json:"rows"`
}
type Row struct {
	Elements []Element `json:"elements"`
}
type Element struct {
	Distance *Distance `json:"distance"`
	Duration *Duration `json:"duration"`
}
type Distance struct {
	Value int `json:"value"`
}
type Duration struct {
	Value int `json:"value"`
}

// train directions

type DirectionsResponse struct {
	Routes []Route `json:"routes"`
}
type Route struct {
	Legs []Leg `json:"legs"`
}
type Leg struct {
	Steps []Step `json:"steps"`
}
type Step struct {
	Distance       *Distance       `json:"distance"`
	Duration       *Duration       `json:"duration"`
	TravelMode     string          `json:"travel_mode"`
	TransitDetails *TransitDetails `json:"transit_details"`
}
type TransitDetails struct {
	ArrivalStop   *Stop        `json:"arrival_stop"`
	DepartureStop *Stop        `json:"departure_stop"`
	DepartureTime *Time        `json:"departure_time"`
	Line          *TransitLine `json:"line"`
}
type Stop struct {
	Location *GoogleMapsLocation `json:"location"`
	Name     string              `json:"name"`
}
type GoogleMapsLocation struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}
type Time struct {
	Value int64 `json:"value"`
}
type TransitLine struct {
	Name      string   `json:"name"`
	ShortName string   `json:"short_name"`
	Vehicle   *Vehicle `json:"vehicle"`
}
type Vehicle struct {
	Type string `json:"type"`
}

// geocoding response

type GeocodeResponse struct {
	Results []Result `json:"results"`
}
type Result struct {
	AddressComponents []AddressComponent `json:"address_components"`
}
type AddressComponent struct {
	LongName  string   `json:"long_name"`
	ShortName string   `json:"short_name"`
	Types     []string `json:"types"`
}

func InitGoogleMapsApi() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	googleApiKey = os.Getenv("GOOGLE_MAPS_API_KEY")
}

func GetDirectionsBike(originCity, destinationCity model.City, date time.Time, hour time.Time, isOutbound bool) ([]model.Segment, error) {
	// distance matrix api url
	baseURL := "https://maps.googleapis.com/maps/api/distancematrix/json"

	// compute departure-time value
	dateHour := time.Date(date.Year(), date.Month(), date.Day(), hour.Hour(), hour.Minute(), 0, 0, hour.Location())
	timestamp := dateHour.Unix()
	departureTime := strconv.FormatInt(timestamp, 10)

	params := url.Values{}
	params.Add("origins", originCity.CityName)
	params.Add("destinations", destinationCity.CityName)
	params.Add("departure-time", departureTime)
	params.Add("mode", "bicycling")
	params.Add("key", googleApiKey)

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	start := time.Now()

	resp, err := http.Get(fullURL)
	if err != nil {
		log.Println("error creating the request: ", err)
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	elapsed := time.Since(start)
	log.Println("CALL Google Maps API Bike took: ", elapsed)

	start = time.Now()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading the body: ", err)
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	var response DistanceMatrixResponse

	jsonReader := bytes.NewReader(body)
	decoder := json.NewDecoder(jsonReader)
	err = decoder.Decode(&response)
	if err != nil {
		log.Println("error decoding JSON: ", err)
		return nil, err
	}

	if len(response.Rows) == 0 ||
		len(response.Rows[0].Elements) == 0 ||
		len(response.OriginAddresses) == 0 ||
		len(response.DestinationAddresses) == 0 ||
		response.Rows[0].Elements[0].Distance == nil ||
		response.Rows[0].Elements[0].Duration == nil {
		log.Println("Missing data in the response")
		return nil, fmt.Errorf("missing data in response")
	}
	distance := response.Rows[0].Elements[0].Distance.Value / 1000

	departureCountry := ""
	if originCity.CountryName != nil {
		departureCountry = *originCity.CountryName
	}
	destinationCountry := ""
	if destinationCity.CountryName != nil {
		destinationCountry = *destinationCity.CountryName
	}

	unifiedTime := time.Date(date.Year(), date.Month(), date.Day(), hour.Hour(), hour.Minute(), 0, 0, hour.Location())

	segment := model.Segment{
		// id auto increment
		DepartureId:        originCity.CityID,
		DestinationId:      destinationCity.CityID,
		DepartureCity:      originCity.CityName,
		DepartureCountry:   departureCountry,
		DestinationCity:    destinationCity.CityName,
		DestinationCountry: destinationCountry,
		DateTime:           unifiedTime,
		Duration:           time.Duration(response.Rows[0].Elements[0].Duration.Value * int(time.Second)),
		Vehicle:            "bike",
		Description:        "",
		Price:              0,
		CO2Emitted:         0,
		Distance:           float64(distance),
		NumSegment:         1,
		IsOutward:          isOutbound,
		TravelID:           -1,
	}

	elapsed = time.Since(start)
	log.Println("ANALYZING Google Maps API Bike took: ", elapsed)

	return []model.Segment{segment}, nil
}

func GetDirectionsCar(originCity, destinationCity model.City, date time.Time, hour time.Time, isOutbound bool) ([]model.Segment, error) {
	// distance matrix api url
	baseURL := "https://maps.googleapis.com/maps/api/distancematrix/json"

	// compute departure-time value
	dateHour := time.Date(date.Year(), date.Month(), date.Day(), hour.Hour(), hour.Minute(), 0, 0, hour.Location())
	timestamp := dateHour.Unix()
	departureTime := strconv.FormatInt(timestamp, 10)

	params := url.Values{}
	params.Add("origins", originCity.CityName)
	params.Add("destinations", destinationCity.CityName)
	params.Add("departure-time", departureTime)
	params.Add("key", googleApiKey)

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	start := time.Now()

	resp, err := http.Get(fullURL)
	if err != nil {
		log.Println("error creating the request: ", err)
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	elapsed := time.Since(start)
	log.Println("CALL Google Maps API Car took: ", elapsed)

	start = time.Now()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading the body: ", err)
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	var response DistanceMatrixResponse

	jsonReader := bytes.NewReader(body)
	decoder := json.NewDecoder(jsonReader)
	err = decoder.Decode(&response)
	if err != nil {
		log.Println("error decoding JSON: ", err)
		return nil, err
	}

	if len(response.Rows) == 0 ||
		len(response.Rows[0].Elements) == 0 ||
		len(response.OriginAddresses) == 0 ||
		len(response.DestinationAddresses) == 0 ||
		response.Rows[0].Elements[0].Distance == nil ||
		response.Rows[0].Elements[0].Duration == nil {
		log.Println("Missing data in the response")
		return nil, fmt.Errorf("missing data in response")
	}

	distance := response.Rows[0].Elements[0].Distance.Value / 1000
	fuelCostPerLiter := GetFuelCostPerLiter(originCity.CityName)
	tollCost := GetTollCost(originCity.CityName, destinationCity.CityName, distance)

	departureCountry := ""
	if originCity.CountryName != nil {
		departureCountry = *originCity.CountryName
	}
	destinationCountry := ""
	if destinationCity.CountryName != nil {
		destinationCountry = *destinationCity.CountryName
	}

	unifiedTime := time.Date(date.Year(), date.Month(), date.Day(), hour.Hour(), hour.Minute(), 0, 0, hour.Location())

	segment := model.Segment{
		// id auto increment
		DepartureId:        originCity.CityID,
		DestinationId:      destinationCity.CityID,
		DepartureCity:      originCity.CityName,
		DepartureCountry:   departureCountry,
		DestinationCity:    destinationCity.CityName,
		DestinationCountry: destinationCountry,
		DateTime:           unifiedTime,
		Duration:           time.Duration(response.Rows[0].Elements[0].Duration.Value * int(time.Second)),
		Vehicle:            "car",
		Description:        "",
		Price:              internals.ComputeCarPrice(fuelCostPerLiter, float64(distance), tollCost),
		CO2Emitted:         internals.ComputeCarEmission(distance),
		Distance:           float64(response.Rows[0].Elements[0].Distance.Value) / 1000,
		NumSegment:         1,
		IsOutward:          isOutbound,
		TravelID:           -1,
	}

	elapsed = time.Since(start)
	log.Println("ANALYZING Google Maps API Car took: ", elapsed)

	return []model.Segment{segment}, nil
}

func GetDirectionsTrain(originCity, destinationCity model.City, date, hour time.Time, isOutbound bool) ([]model.Segment, error) {
	// google directions api
	baseURL := "https://maps.googleapis.com/maps/api/directions/json"

	// compute departure-time value
	dateHour := time.Date(date.Year(), date.Month(), date.Day(), hour.Hour(), hour.Minute(), 0, 0, hour.Location())
	timestamp := dateHour.Unix()

	params := url.Values{}
	params.Add("origin", originCity.CityName)
	params.Add("destination", destinationCity.CityName)
	params.Add("mode", "transit")
	params.Add("transit_mode", "rail")
	params.Add("departure_time", strconv.FormatInt(timestamp, 10))
	params.Add("transit_routing_preference", "fewer_transfers")
	params.Add("key", googleApiKey)

	params.Add("alternatives", "false")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	start := time.Now()

	resp, err := http.Get(fullURL)
	if err != nil {
		log.Println("error creating the request: ", err)
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	elapsed := time.Since(start)
	log.Println("CALL Google Maps API Train took: ", elapsed)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading the body: ", err)
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	segments, err := decodeDirectionsTransit(body, originCity, destinationCity, "train", isOutbound)
	if err != nil {
		log.Println("error decoding JSON: ", err)
		return nil, err
	}

	return segments, nil
}

func GetDirectionsBus(originCity, destinationCity model.City, date, hour time.Time, isOutbound bool) ([]model.Segment, error) {
	// google directions api
	baseURL := "https://maps.googleapis.com/maps/api/directions/json"

	// compute departure-time value
	dateHour := time.Date(date.Year(), date.Month(), date.Day(), hour.Hour(), hour.Minute(), 0, 0, hour.Location())
	timestamp := dateHour.Unix()

	// build url
	params := url.Values{}
	params.Add("origin", originCity.CityName)
	params.Add("destination", destinationCity.CityName)
	params.Add("mode", "transit")
	params.Add("transit_mode", "bus")
	params.Add("departure_time", strconv.FormatInt(timestamp, 10))
	params.Add("key", googleApiKey)
	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	start := time.Now()

	// request
	resp, err := http.Get(fullURL)
	if err != nil {
		log.Println("error creating the request: ", err)
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	elapsed := time.Since(start)
	log.Println("CALL Google Maps API Bus took: ", elapsed)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading the body: ", err)
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	segments, err := decodeDirectionsTransit(body, originCity, destinationCity, "bus", isOutbound)
	if err != nil {
		log.Println("error decoding JSON: ", err)
		return nil, err
	}

	return segments, nil
}

func decodeDirectionsTransit(body []byte, originCity, destinationCity model.City, transitMode string, isOutbound bool) ([]model.Segment, error) {

	start := time.Now()

	// vehicles returned by the api

	busVehicles := []string{"BUS", "INTERCITY_BUS", "SHARE_TAXI", "TROLLEYBUS"}
	trainVehicles := []string{"COMMUTER_TRAIN", "HEAVY_RAIL", "HIGH_SPEED_TRAIN", "LONG_DISTANCE_TRAIN", "METRO_RAIL", "MONORAIL", "RAIL", "SUBWAY", "TRAM"}

	var response DirectionsResponse

	jsonReader := bytes.NewReader(body)
	decoder := json.NewDecoder(jsonReader)
	err := decoder.Decode(&response)
	if err != nil {
		log.Println("Error decoding json response: ", err)
		return nil, err
	}

	// check no missing data
	if len(response.Routes) == 0 ||
		len(response.Routes[0].Legs) == 0 ||
		len(response.Routes[0].Legs[0].Steps) == 0 {
		log.Println("Missing data in the response")
		return nil, fmt.Errorf("missing data in response")
	}

	leg := response.Routes[0].Legs[0]

	var segments []model.Segment
	numSegment := 0
	for _, step := range leg.Steps {
		var segment model.Segment
		numSegment++

		if step.TravelMode == "WALKING" {
			duration := 0
			if step.Duration != nil {
				duration = step.Duration.Value
			}

			distance := 0.0
			if step.Distance != nil {
				distance = float64(step.Distance.Value)
			}

			segment = model.Segment{
				// id is autoincrement
				DepartureId:        -1, // updated at the end
				DestinationId:      -1, // updated at the end
				DepartureCity:      "", // updated at the end
				DestinationCity:    "", // updated at the end
				DepartureCountry:   "", // updated at the end
				DestinationCountry: "", // updated at the end
				DateTime:           time.Unix(0, 0),
				Duration:           time.Duration(duration) * time.Second,
				Vehicle:            "walk",
				Description:        "",
				Price:              0,
				Distance:           distance,
				CO2Emitted:         0,
				NumSegment:         numSegment,
				IsOutward:          isOutbound,
				// travel id set later
			}
		} else {
			// check no missing step data
			if step.TravelMode == "" ||
				step.TransitDetails == nil ||
				step.TransitDetails.Line == nil ||
				step.TransitDetails.Line.Vehicle == nil ||
				step.TransitDetails.DepartureTime == nil ||
				step.TransitDetails.DepartureStop == nil ||
				step.TransitDetails.ArrivalStop == nil ||
				step.TransitDetails.DepartureStop.Location == nil ||
				step.TransitDetails.ArrivalStop.Location == nil ||
				step.Distance == nil ||
				step.Duration == nil {
				log.Println("Missing data in the response ------- FOR A STEP")
				return nil, fmt.Errorf("missing data in response ------- FOR A STEP")
			}

			// check segment travel mode
			travelMode := ""
			found := false
			if transitMode == "bus" {
				for _, vehicle := range busVehicles {
					if step.TransitDetails.Line.Vehicle.Type == vehicle {
						found = true
						travelMode = transitMode
					}
				}
			} else if transitMode == "train" {
				for _, vehicle := range trainVehicles {
					if step.TransitDetails.Line.Vehicle.Type == vehicle {
						found = true
						travelMode = transitMode
					}
				}
			}
			if !found {
				return nil, fmt.Errorf("provided data has wrong transit mode")
			}

			returnedTime := time.Unix(step.TransitDetails.DepartureTime.Value, 0)
			distance := float64(step.Distance.Value) / 1000

			stepDepCity, err1 := GetCityNoIata(
				step.TransitDetails.DepartureStop.Name,
				step.TransitDetails.DepartureStop.Location.Latitude,
				step.TransitDetails.DepartureStop.Location.Longitude)
			if err1 != nil {
				return nil, err1
			}
			stepDestCity, err1 := GetCityNoIata(
				step.TransitDetails.ArrivalStop.Name,
				step.TransitDetails.ArrivalStop.Location.Latitude,
				step.TransitDetails.ArrivalStop.Location.Longitude)
			if err1 != nil {
				return nil, err1
			}

			co2Emitted := 0.0
			if transitMode == "train" {
				co2Emitted = internals.ComputeTrainEmission(int(distance))
			} else if transitMode == "bus" {
				co2Emitted = internals.ComputeBusEmission(int(distance))
			}

			departureCountry := ""
			if stepDepCity.CountryName != nil {
				departureCountry = *stepDepCity.CountryName
			}
			destinationCountry := ""
			if stepDestCity.CountryName != nil {
				destinationCountry = *stepDestCity.CountryName
			}
			segment = model.Segment{
				// id is autoincrement
				DepartureId:        stepDepCity.CityID,
				DestinationId:      stepDestCity.CityID,
				DepartureCity:      stepDepCity.CityName,
				DepartureCountry:   departureCountry,
				DestinationCity:    stepDestCity.CityName,
				DestinationCountry: destinationCountry,
				DateTime:           time.Date(returnedTime.Year(), returnedTime.Month(), returnedTime.Day(), returnedTime.Hour(), returnedTime.Minute(), returnedTime.Second(), returnedTime.Nanosecond(), returnedTime.Location()),
				Duration:           time.Duration(step.Duration.Value) * time.Second,
				Vehicle:            travelMode,
				Description:        step.TransitDetails.Line.ShortName + ", " + step.TransitDetails.Line.Name,
				Price:              GetTransitCost(stepDepCity.CityName, stepDestCity.CityName, transitMode, int(distance)),
				Distance:           distance,
				CO2Emitted:         co2Emitted,
				NumSegment:         numSegment,
				IsOutward:          isOutbound,
				// travel id set later
			}
		}

		// append to list
		segments = append(segments, segment)
	}

	segments = resetDepDestCity(segments, originCity, destinationCity)
	segments = compactTransitSegments(segments)
	segments = setMissingDataWalkingSegments(segments, originCity, destinationCity)

	elapsed := time.Since(start)
	log.Println("ANALYZING Google Maps API Train/Bus took: ", elapsed)

	return segments, nil
}

// method that sets the first city and the last city to origin and destination city (first class cities)
func resetDepDestCity(segments []model.Segment, originCity, destinationCity model.City) []model.Segment {
	// set departure
	for i, segment := range segments {
		if segment.Vehicle != "walk" {
			segments[i].DepartureId = originCity.CityID
			segments[i].DepartureCity = originCity.CityName
			if originCity.CountryName != nil {
				segments[i].DepartureCountry = *originCity.CountryName
			} else {
				segments[i].DepartureCountry = ""
			}

			// exit after the first non-walking segment
			break
		}
	}

	// set destination
	for i := len(segments) - 1; i >= 0; i-- {
		if segments[i].Vehicle != "walk" {
			segments[i].DestinationId = destinationCity.CityID

			segments[i].DestinationCity = destinationCity.CityName
			if destinationCity.CountryName != nil {
				segments[i].DestinationCountry = *destinationCity.CountryName
			} else {
				segments[i].DestinationCountry = ""
			}
			// exit after the first non-walking segment
			break
		}
	}

	return segments
}

func compactTransitSegments(segments []model.Segment) []model.Segment {
	var compactedSegments []model.Segment

	lastIsWalk := false
	totDuration := time.Duration(0)
	totDistance := 0.0
	isOutward := false
	for i := 0; i < len(segments); i++ {
		if segments[i].Vehicle == "walk" {
			lastIsWalk = true
			isOutward = segments[i].IsOutward
			totDuration += segments[i].Duration
			totDistance += segments[i].Distance
		} else {
			// add compressed walking segment
			if lastIsWalk {
				compactedSegments = append(compactedSegments, model.Segment{
					// id is autoincrement
					DepartureId:        -1, // updated at the end
					DestinationId:      -1, // updated at the end
					DepartureCity:      "", // updated at the end
					DepartureCountry:   "", // updated at the end
					DestinationCity:    "", // updated at the end
					DestinationCountry: "", // updated at the end
					DateTime:           time.Unix(0, 0),
					Duration:           totDuration,
					Vehicle:            "walk",
					Description:        "",
					Price:              0,
					Distance:           totDistance,
					CO2Emitted:         0,
					NumSegment:         -1,
					IsOutward:          isOutward,
					// travel id set later
				})

				// reset values
				lastIsWalk = false
				totDuration = time.Duration(0)
				totDistance = 0.0
				isOutward = false
			}

			// add non walking segment
			compactedSegments = append(compactedSegments, segments[i])
		}
	}

	// reset num segment
	for i := 0; i < len(compactedSegments); i++ {
		compactedSegments[i].NumSegment = i + 1
	}

	return compactedSegments
}

func setMissingDataWalkingSegments(segments []model.Segment, originCity, destinationCity model.City) []model.Segment {
	for i, _ := range segments {
		if segments[i].Vehicle == "walk" {
			// set departure and destination city id
			if segments[i].NumSegment == 1 {
				segments[i].DepartureId = originCity.CityID
			} else {
				segments[i].DepartureId = segments[i-1].DestinationId
			}
			if segments[i].NumSegment == len(segments) {
				segments[i].DestinationId = destinationCity.CityID
			} else {
				segments[i].DestinationId = segments[i+1].DepartureId
			}

			// compute date and hour
			if segments[i].NumSegment == len(segments) {
				// if last segment, based on previous segment (if present)
				if segments[i].NumSegment > 1 {
					prevTime := segments[i-1].DateTime
					prevDuration := segments[i-1].Duration
					walkDepTime := prevTime.Add(prevDuration)
					segments[i].DateTime = walkDepTime
				}
			} else {
				// if not the last segment, based on the next segment
				nextTime := segments[i+1].DateTime
				walkDuration := segments[i].Duration
				walkDepTime := nextTime.Add(-walkDuration)
				segments[i].DateTime = walkDepTime
			}
		}
		// else don't modify
	}
	return segments
}

func GetCityNoIata(cityName string, latitude, longitude float64) (model.City, error) {
	// get country associated to city (place more in general) and coordinates
	countryName := ""
	countryCode := ""

	// google geocoding api
	baseURL := "https://maps.googleapis.com/maps/api/geocode/json"

	latitudeString := strconv.FormatFloat(latitude, 'f', -1, 64)
	longitudeString := strconv.FormatFloat(longitude, 'f', -1, 64)
	latlngString := latitudeString + "," + longitudeString

	params := url.Values{}
	params.Add("latlng", latlngString)
	params.Add("key", googleApiKey)

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	resp, err := http.Get(fullURL)
	if err != nil {
		log.Println("error creating the request: ", err)
		return model.City{}, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading the body: ", err)
		return model.City{}, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return model.City{}, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	var response GeocodeResponse
	jsonReader := bytes.NewReader(body)
	decoder := json.NewDecoder(jsonReader)
	err = decoder.Decode(&response)
	if err != nil {
		log.Println("error decoding JSON: ", err)
		return model.City{}, err
	}

	for _, result := range response.Results {
		for _, component := range result.AddressComponents {
			for _, t := range component.Types {
				if t == "country" {
					countryName = component.LongName
					countryCode = component.ShortName
					break
				}
			}
		}
	}

	cityDAO := db.NewCityDAO(db.GetDB())

	// check if a city with same name and country exists
	city, err := cityDAO.GetCityByNameAndCountry(cityName, countryName)
	if err == nil {
		return city, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.City{}, err
	}

	// else create a city without iata
	city = model.City{
		// id autogenerated
		CityIata:    nil,
		CityName:    cityName,
		CountryName: &countryName,
		CountryCode: &countryCode,
		Continent:   nil,
	}

	err = cityDAO.CreateCity(&city)
	if err != nil {
		return model.City{}, err
	}

	return city, nil
}
