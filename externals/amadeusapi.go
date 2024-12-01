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
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var accessToken string
var amadeusApiKey string
var amadeusApiSecret string

// firebase authentication structure

type AuthResponse struct {
	AccessToken string `json:"access_token"`
}

// amadeus structures

type FlightResponse struct {
	Data []FlightOffer `json:"data"`
}
type FlightOffer struct {
	Itineraries []Itinerary  `json:"itineraries"`
	Price       *FlightPrice `json:"price"`
}
type FlightPrice struct {
	GrandTotal string `json:"grandTotal"`
}
type Itinerary struct {
	Segments []FlightSegment `json:"segments"`
}
type FlightSegment struct {
	Departure   *Airport `json:"departure"`
	Arrival     *Airport `json:"arrival"`
	CarrierCode string   `json:"carrierCode"`
	Number      string   `json:"number"`
	Duration    string   `json:"duration"`
}
type Airport struct {
	IataCode string `json:"iataCode"`
	At       string `json:"at"`
}
type LocationResponse struct {
	Data []AmadeusLocation `json:"data"`
}
type AmadeusLocation struct {
	Name     string   `json:"name"`
	IataCode *string  `json:"iataCode"`
	SubType  string   `json:"subType"`
	Address  *Address `json:"address"`
	GeoCode  *GeoCode `json:"geoCode"`
}
type Address struct {
	CityCode    *string `json:"cityCode"`
	CountryCode *string `json:"countryCode"`
}
type GeoCode struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

func InitAmadeusApi() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	amadeusApiKey = os.Getenv("AMADEUS_API_KEY")
	amadeusApiSecret = os.Getenv("AMADEUS_API_SECRET")
}

func GetAccessToken() error {
	// access token url
	accessTokenUrl := "https://test.api.amadeus.com/v1/security/oauth2/token"

	// create POST request
	payload := []byte("grant_type=client_credentials&client_id=" + amadeusApiKey + "&client_secret=" + amadeusApiSecret)
	req, err := http.NewRequest("POST", accessTokenUrl, bytes.NewBuffer(payload))
	if err != nil {
		log.Println("Error creating request: ", err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error creating request: ", err)
		return err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	// check response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading body: ", err)
		return err
	}

	// save access token
	var authResponse AuthResponse
	err = json.Unmarshal(body, &authResponse)
	if err != nil {
		log.Println("Error unmarshalling auth response: ", err)
		return err
	}
	accessToken = authResponse.AccessToken

	return nil
}

func GetFlights(departureCity, destinationCity model.City, date time.Time, isOutbound bool) ([][]model.Segment, error) {
	// get cities iata codes
	if departureCity.CityIata == nil {
		return nil, fmt.Errorf("null departure city iata")
	}
	originIata := *departureCity.CityIata
	if destinationCity.CityIata == nil {
		return nil, fmt.Errorf("null destination city iata")
	}
	destinationIata := *destinationCity.CityIata

	// amadeus flight offers search url
	baseUrl := "https://test.api.amadeus.com/v2/shopping/flight-offers"

	// compute departure-time value
	departureDate := date.Format("2006-01-02")

	// compose url
	params := url.Values{}
	params.Add("originLocationCode", originIata)
	params.Add("destinationLocationCode", destinationIata)
	params.Add("departureDate", departureDate)
	params.Add("adults", "1")
	params.Add("max", "3")

	apiUrl := fmt.Sprintf("%s?%s", baseUrl, params.Encode())

	// create request
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{}

	start := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	// if access token out of date
	if resp == nil || resp.StatusCode == http.StatusUnauthorized {
		// get new access token
		err = GetAccessToken()
		if err != nil {
			log.Println("Failed to get amadeus api access token: ", err)
			return nil, err
		}

		// repeat request
		var req2 *http.Request
		req2, err = http.NewRequest("GET", apiUrl, nil)
		if err != nil {
			return nil, err
		}
		req2.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = resp.Body.Close()
			if err != nil {
				log.Println("Error closing response body:", err)
			}
		}()
		if resp == nil || resp.StatusCode == http.StatusUnauthorized {
			return nil, errors.New("unauthorized")
		}
	}

	elapsed := time.Since(start)
	fmt.Println("Amadeus API took", elapsed)

	start = time.Now()

	// check response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	var response FlightResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Print("Error while decoding body: ", err)
		return nil, err
	}

	if len(response.Data) == 0 {
		log.Println("Missing data in the response")
		return nil, fmt.Errorf("missing data in the response")
	}

	var flights [][]model.Segment

	for _, flightOffer := range response.Data {
		// check no missing data
		if flightOffer.Itineraries == nil ||
			len(flightOffer.Itineraries) == 0 ||
			flightOffer.Itineraries[0].Segments == nil ||
			len(flightOffer.Itineraries[0].Segments) == 0 {
			// a different offer might have data
			// don't return, just skip an iteration
			continue
		}
		itinerary := flightOffer.Itineraries[0]

		var flight []model.Segment
		// if I break out of the loop, I must not add to flights
		addToFlights := true
		numSegment := 0
		var distances []float64
		for _, flightSegment := range itinerary.Segments {
			// check segment data
			if flightSegment.Departure == nil ||
				flightSegment.Arrival == nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}

			// increment segment number
			numSegment++
			// get departure time
			parsedTime, err1 := time.Parse("2006-01-02T15:04:05", flightSegment.Departure.At)
			if err1 != nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}
			// get duration
			duration, err1 := parseISODuration(flightSegment.Duration)
			if err1 != nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}
			// get cities
			segmentDepCity, segmentDepAirport, err1 := GetCityAndAirportFromAirportIATA(flightSegment.Departure.IataCode)
			if err1 != nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}
			segmentDestCity, segmentDestAirport, err1 := GetCityAndAirportFromAirportIATA(flightSegment.Arrival.IataCode)
			if err1 != nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}
			// compute Haversine distance
			distance := internals.ComputeHaversineDistance(segmentDepAirport.Latitude, segmentDepAirport.Longitude, segmentDestAirport.Latitude, segmentDestAirport.Longitude)
			distances = append(distances, distance)

			departureCountry := ""
			if segmentDepCity.CountryName != nil {
				departureCountry = *segmentDepCity.CountryName
			}
			destinationCountry := ""
			if segmentDestCity.CountryName != nil {
				destinationCountry = *segmentDestCity.CountryName
			}
			segment := model.Segment{
				// segment id is autogenerated
				DepartureId:        segmentDepCity.CityID,
				DestinationId:      segmentDestCity.CityID,
				DepartureCity:      segmentDepCity.CityName,
				DepartureCountry:   departureCountry,
				DestinationCity:    segmentDestCity.CityName,
				DestinationCountry: destinationCountry,
				DateTime:           parsedTime,
				Duration:           duration,
				Vehicle:            "plane",
				Description:        flightSegment.CarrierCode + " " + flightSegment.Number,
				// indicative price set after
				CO2Emitted: internals.ComputeAircraftEmission(int(duration.Hours()), int(duration.Minutes())),
				Distance:   distance,
				NumSegment: numSegment,
				IsOutward:  isOutbound,
				// travel id can't be set here
			}
			flight = append(flight, segment)
		}

		if addToFlights {
			// set indicative price to segments
			totalPrice, err1 := strconv.ParseFloat(flightOffer.Price.GrandTotal, 64)
			if err1 != nil {
				totalPrice = 0
			}
			totalDistance := 0.0
			for _, d := range distances {
				totalDistance += d
			}
			for i := range flight {
				flight[i].Price = totalPrice * (distances[i] / totalDistance)
			}

			flights = append(flights, flight)
		}
	}

	elapsed = time.Since(start)
	fmt.Println("ANALYZING Amadeus API flights took: ", elapsed)

	return flights, nil
}

func parseISODuration(duration string) (time.Duration, error) {
	re := regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)
	matches := re.FindStringSubmatch(duration)

	if matches == nil {
		return 0, fmt.Errorf("wrong format for duration")
	}

	var hours, minutes, seconds int
	if matches[1] != "" {
		hours, _ = strconv.Atoi(matches[1])
	}
	if matches[2] != "" {
		minutes, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		seconds, _ = strconv.Atoi(matches[3])
	}

	totalDuration := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	return totalDuration, nil
}

func GetCityAndAirportFromAirportIATA(iata string) (model.City, model.Airport, error) {
	// this method returns the city based on the airport_iata

	cityDAO := db.NewCityDAO(db.GetDB())

	// check existing airport_iata
	airport, err1 := cityDAO.GetAirportByAirportIata(iata)
	city, err2 := cityDAO.GetCityByAirportIata(iata)
	if err1 == nil && err2 == nil {
		return city, airport, nil
	} else if !errors.Is(err1, gorm.ErrRecordNotFound) {
		return model.City{}, model.Airport{}, err1
	} else if !errors.Is(err2, gorm.ErrRecordNotFound) {
		return model.City{}, model.Airport{}, err2
	}

	// else, make an api call
	err := MakeAirportCityCall(iata)
	if err != nil {
		return model.City{}, model.Airport{}, err
	}

	// try getting by airport_iata
	airport, err1 = cityDAO.GetAirportByAirportIata(iata)
	city, err2 = cityDAO.GetCityByAirportIata(iata)
	if err1 == nil && err2 == nil {
		return city, airport, nil
	} else {
		// after api call, the iata is not present in the db
		return model.City{}, model.Airport{}, err1
	}
}

func MakeAirportCityCall(keyword string) error {
	apiUrl := "https://test.api.amadeus.com/v1/reference-data/locations?subType=CITY,AIRPORT&keyword=" + keyword

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		log.Println("Error while creating the request: ", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error while creating the request: ", err)
		return err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	// if access token out of date
	if resp == nil || resp.StatusCode == http.StatusUnauthorized {
		// get new access token
		err = GetAccessToken()
		if err != nil {
			log.Println("Failed to get amadeus api access token: ", err)
			return err
		}

		// repeat request
		var req2 *http.Request
		req2, err = http.NewRequest("GET", apiUrl, nil)
		if err != nil {
			log.Println("Error creating the request:", err)
			return err
		}
		req2.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err = client.Do(req2)
		if err != nil {
			log.Println("Error while creating the request: ", err)
			return err
		}
		defer func() {
			err = resp.Body.Close()
			if err != nil {
				log.Println("Error closing response body:", err)
			}
		}()
		if resp == nil || resp.StatusCode == http.StatusUnauthorized {
			return errors.New("unauthorized")
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error while reading the body: ", err)
		return err
	}

	var locationResponse LocationResponse
	err = json.Unmarshal(body, &locationResponse)
	if err != nil {
		log.Println("Error parsing JSON: ", err)
		return err
	}

	if locationResponse.Data == nil ||
		len(locationResponse.Data) == 0 {
		return fmt.Errorf("no data in the response")
	}

	cityDAO := db.NewCityDAO(db.GetDB())

	// add airports
	for _, element := range locationResponse.Data {
		if element.Address == nil ||
			element.Address.CityCode == nil ||
			element.Address.CountryCode == nil ||
			element.GeoCode == nil ||
			element.GeoCode.Latitude == nil ||
			element.GeoCode.Longitude == nil ||
			element.SubType == "" {
			// skip only one airport
			continue
		}

		if element.SubType == "AIRPORT" {
			// check airport not present
			airport, err1 := cityDAO.GetAirportByAirportIata(*element.IataCode)
			if err1 == nil {
				// airport already present
				continue
			}

			// check corresponding city exists in db with same city_iata and country_code
			cityIata := *element.Address.CityCode
			countryCode := *element.Address.CountryCode
			dbCity, err1 := cityDAO.GetCityByIataAndCountryCode(cityIata, countryCode)
			if err1 != nil {
				// skip only one airport
				continue
			}

			// create airport
			airport = model.Airport{
				// airport id is autogenerated
				AirportName: capitalizeFirstLetter(element.Name),
				AirportIata: *element.IataCode,
				Latitude:    *element.GeoCode.Latitude,
				Longitude:   *element.GeoCode.Longitude,
				CityID:      dbCity.CityID,
			}

			// add airport to db
			err1 = cityDAO.CreateAirport(&airport)
			if err1 != nil {
				continue
			}
		}
	}

	return nil
}

func capitalizeFirstLetter(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			// get runes
			runes := []rune(word)

			// first letter uppercase
			runes[0] = unicode.ToUpper(runes[0])

			// other letters lowercase
			for j, letter := range runes {
				if j > 0 {
					runes[j] = unicode.ToLower(letter)
				}
			}

			// save new word
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
