package externals

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"green-journey-server/internals"
	"green-journey-server/model"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"
)

var accessToken string
var amadeusApiKey string
var amadeusApiSecret string

type AuthResponse struct {
	AccessToken string `json:"access_token"`
}

type FlightResponse struct {
	Meta *Meta         `json:"meta"`
	Data []FlightOffer `json:"data"`
}

type Meta struct {
	Count int `json:"count"`
	Links struct {
		Self string `json:"self"`
	} `json:"links"`
}

type FlightOffer struct {
	Type                     string            `json:"type"`
	Id                       string            `json:"id"`
	Source                   string            `json:"source"`
	InstantTicketingRequired bool              `json:"instantTicketingRequired"`
	Itineraries              []Itinerary       `json:"itineraries"`
	Price                    *FlightPrice      `json:"price"`
	TravelerPricings         []TravelerPricing `json:"travelerPricings"`
}

type TravelerPricing struct {
	TravelerId string       `json:"travelerId"`
	Price      *FlightPrice `json:"price"`
}

type FlightPrice struct {
	Currency   string `json:"currency"`
	Total      string `json:"total"`
	Base       string `json:"base"`
	GrandTotal string `json:"grandTotal"`
}

type Itinerary struct {
	Duration string          `json:"duration"`
	Segments []FlightSegment `json:"segments"`
}

type FlightSegment struct {
	Departure   *Airport `json:"departure"`
	Arrival     *Airport `json:"arrival"`
	CarrierCode string   `json:"carrierCode"`
	Number      string   `json:"number"`
	Aircraft    struct {
		Code string `json:"code"`
	} `json:"aircraft"`
	Operating struct {
		CarrierCode string `json:"carrierCode"`
	} `json:"operating"`
	Duration        string `json:"duration"`
	Id              string `json:"id"`
	NumberOfStops   int    `json:"numberOfStops"`
	BlacklistedInEU bool   `json:"blacklistedInEU"`
}

type Airport struct {
	IataCode string `json:"iataCode"`
	Terminal string `json:"terminal"`
	At       string `json:"at"`
}

type LocationResponse struct {
	Data []Location `json:"data"`
}

type Location struct {
	Name     string   `json:"name"`
	IataCode string   `json:"iataCode"`
	SubType  string   `json:"subType"`
	Address  *Address `json:"address"`
	GeoCode  *GeoCode `json:"geoCode"`
}

type Address struct {
	CityName string `json:"cityName"`
}

type GeoCode struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
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

func GetFlights(origin, destination string, date time.Time, isOutbound bool) ([][]model.Segment, error) {
	// amadeus flight offers search url
	baseUrl := "https://test.api.amadeus.com/v2/shopping/flight-offers"

	// compute IATA codes
	originIATA, err := GetIATAFromCity(origin)
	if err != nil {
		fmt.Println("Error getting IATA from city: ", err)
		return nil, err
	}
	destinationIATA, err := GetIATAFromCity(destination)
	if err != nil {
		fmt.Println("Error getting IATA from city: ", err)
		return nil, err
	}

	// compute departure-time value
	departureDate := date.Format("2006-01-02")

	// compose url
	params := url.Values{}
	params.Add("originLocationCode", originIATA)
	params.Add("destinationLocationCode", destinationIATA)
	params.Add("departureDate", departureDate)
	params.Add("adults", "1")
	params.Add("max", "3")

	apiUrl := fmt.Sprintf("%s?%s", baseUrl, params.Encode())

	// create request
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		fmt.Println("Error creating request: ", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error creating request: ", err)
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
			fmt.Println("Error creating request: ", err)
			return nil, err
		}
		req2.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err = client.Do(req)
		if err != nil {
			fmt.Println("Error creating request: ", err)
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

	// check response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading body: ", err)
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	// TODO remove
	//body := []byte("{\n   \"meta\":{\n      \"count\":3,\n      \"links\":{\n         \"self\":\"https://test.api.amadeus.com/v2/shopping/flight-offers?originLocationCode=MXP&destinationLocationCode=PAR&departureDate=2024-10-10&adults=1&max=3\"\n      }\n   },\n   \"data\":[\n      {\n         \"type\":\"flight-offer\",\n         \"id\":\"1\",\n         \"source\":\"GDS\",\n         \"instantTicketingRequired\":false,\n         \"nonHomogeneous\":false,\n         \"oneWay\":false,\n         \"isUpsellOffer\":false,\n         \"lastTicketingDate\":\"2024-10-09\",\n         \"lastTicketingDateTime\":\"2024-10-09\",\n         \"numberOfBookableSeats\":4,\n         \"itineraries\":[\n            {\n               \"duration\":\"PT1H30M\",\n               \"segments\":[\n                  {\n                     \"departure\":{\n                        \"iataCode\":\"MXP\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T09:45:00\"\n                     },\n                     \"arrival\":{\n                        \"iataCode\":\"ORY\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T11:15:00\"\n                     },\n                     \"carrierCode\":\"IB\",\n                     \"number\":\"5775\",\n                     \"aircraft\":{\n                        \"code\":\"320\"\n                     },\n                     \"operating\":{\n                        \"carrierCode\":\"VY\"\n                     },\n                     \"duration\":\"PT1H30M\",\n                     \"id\":\"1\",\n                     \"numberOfStops\":0,\n                     \"blacklistedInEU\":false\n                  }\n               ]\n            }\n         ],\n         \"price\":{\n            \"currency\":\"EUR\",\n            \"total\":\"136.23\",\n            \"base\":\"112.00\",\n            \"fees\":[\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"SUPPLIER\"\n               },\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"TICKETING\"\n               }\n            ],\n            \"grandTotal\":\"136.23\"\n         },\n         \"pricingOptions\":{\n            \"fareType\":[\n               \"PUBLISHED\"\n            ],\n            \"includedCheckedBagsOnly\":true\n         },\n         \"validatingAirlineCodes\":[\n            \"IB\"\n         ],\n         \"travelerPricings\":[\n            {\n               \"travelerId\":\"1\",\n               \"fareOption\":\"STANDARD\",\n               \"travelerType\":\"ADULT\",\n               \"price\":{\n                  \"currency\":\"EUR\",\n                  \"total\":\"136.23\",\n                  \"base\":\"112.00\"\n               },\n               \"fareDetailsBySegment\":[\n                  {\n                     \"segmentId\":\"1\",\n                     \"cabin\":\"ECONOMY\",\n                     \"fareBasis\":\"OOVYTF\",\n                     \"brandedFare\":\"TIMEFLEXVY\",\n                     \"brandedFareLabel\":\"TIMEFLEX PLUS VUELING\",\n                     \"class\":\"O\",\n                     \"includedCheckedBags\":{\n                        \"quantity\":1\n                     },\n                     \"amenities\":[\n                        {\n                           \"description\":\"SECOND CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"THIRD CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"SNACK OR DRINK\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"MEAL\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"WIFI CONNECTION\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"TRAVEL_SERVICES\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        }\n                     ]\n                  }\n               ]\n            }\n         ]\n      },\n      {\n         \"type\":\"flight-offer\",\n         \"id\":\"2\",\n         \"source\":\"GDS\",\n         \"instantTicketingRequired\":false,\n         \"nonHomogeneous\":false,\n         \"oneWay\":false,\n         \"isUpsellOffer\":false,\n         \"lastTicketingDate\":\"2024-10-09\",\n         \"lastTicketingDateTime\":\"2024-10-09\",\n         \"numberOfBookableSeats\":4,\n         \"itineraries\":[\n            {\n               \"duration\":\"PT1H30M\",\n               \"segments\":[\n                  {\n                     \"departure\":{\n                        \"iataCode\":\"MXP\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T14:55:00\"\n                     },\n                     \"arrival\":{\n                        \"iataCode\":\"ORY\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T16:25:00\"\n                     },\n                     \"carrierCode\":\"IB\",\n                     \"number\":\"5779\",\n                     \"aircraft\":{\n                        \"code\":\"321\"\n                     },\n                     \"operating\":{\n                        \"carrierCode\":\"VY\"\n                     },\n                     \"duration\":\"PT1H30M\",\n                     \"id\":\"2\",\n                     \"numberOfStops\":0,\n                     \"blacklistedInEU\":false\n                  }\n               ]\n            }\n         ],\n         \"price\":{\n            \"currency\":\"EUR\",\n            \"total\":\"136.23\",\n            \"base\":\"112.00\",\n            \"fees\":[\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"SUPPLIER\"\n               },\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"TICKETING\"\n               }\n            ],\n            \"grandTotal\":\"136.23\"\n         },\n         \"pricingOptions\":{\n            \"fareType\":[\n               \"PUBLISHED\"\n            ],\n            \"includedCheckedBagsOnly\":true\n         },\n         \"validatingAirlineCodes\":[\n            \"IB\"\n         ],\n         \"travelerPricings\":[\n            {\n               \"travelerId\":\"1\",\n               \"fareOption\":\"STANDARD\",\n               \"travelerType\":\"ADULT\",\n               \"price\":{\n                  \"currency\":\"EUR\",\n                  \"total\":\"136.23\",\n                  \"base\":\"112.00\"\n               },\n               \"fareDetailsBySegment\":[\n                  {\n                     \"segmentId\":\"2\",\n                     \"cabin\":\"ECONOMY\",\n                     \"fareBasis\":\"OOVYTF\",\n                     \"brandedFare\":\"TIMEFLEXVY\",\n                     \"brandedFareLabel\":\"TIMEFLEX PLUS VUELING\",\n                     \"class\":\"O\",\n                     \"includedCheckedBags\":{\n                        \"quantity\":1\n                     },\n                     \"amenities\":[\n                        {\n                           \"description\":\"SECOND CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"THIRD CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"SNACK OR DRINK\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"MEAL\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"WIFI CONNECTION\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"TRAVEL_SERVICES\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        }\n                     ]\n                  }\n               ]\n            }\n         ]\n      },\n      {\n         \"type\":\"flight-offer\",\n         \"id\":\"3\",\n         \"source\":\"GDS\",\n         \"instantTicketingRequired\":false,\n         \"nonHomogeneous\":false,\n         \"oneWay\":false,\n         \"isUpsellOffer\":false,\n         \"lastTicketingDate\":\"2024-10-09\",\n         \"lastTicketingDateTime\":\"2024-10-09\",\n         \"numberOfBookableSeats\":4,\n         \"itineraries\":[\n            {\n               \"duration\":\"PT1H35M\",\n               \"segments\":[\n                  {\n                     \"departure\":{\n                        \"iataCode\":\"MXP\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T20:10:00\"\n                     },\n                     \"arrival\":{\n                        \"iataCode\":\"ORY\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T21:45:00\"\n                     },\n                     \"carrierCode\":\"IB\",\n                     \"number\":\"5777\",\n                     \"aircraft\":{\n                        \"code\":\"321\"\n                     },\n                     \"operating\":{\n                        \"carrierCode\":\"VY\"\n                     },\n                     \"duration\":\"PT1H35M\",\n                     \"id\":\"3\",\n                     \"numberOfStops\":0,\n                     \"blacklistedInEU\":false\n                  }\n               ]\n            }\n         ],\n         \"price\":{\n            \"currency\":\"EUR\",\n            \"total\":\"136.23\",\n            \"base\":\"112.00\",\n            \"fees\":[\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"SUPPLIER\"\n               },\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"TICKETING\"\n               }\n            ],\n            \"grandTotal\":\"136.23\"\n         },\n         \"pricingOptions\":{\n            \"fareType\":[\n               \"PUBLISHED\"\n            ],\n            \"includedCheckedBagsOnly\":true\n         },\n         \"validatingAirlineCodes\":[\n            \"IB\"\n         ],\n         \"travelerPricings\":[\n            {\n               \"travelerId\":\"1\",\n               \"fareOption\":\"STANDARD\",\n               \"travelerType\":\"ADULT\",\n               \"price\":{\n                  \"currency\":\"EUR\",\n                  \"total\":\"136.23\",\n                  \"base\":\"112.00\"\n               },\n               \"fareDetailsBySegment\":[\n                  {\n                     \"segmentId\":\"3\",\n                     \"cabin\":\"ECONOMY\",\n                     \"fareBasis\":\"OOVYTF\",\n                     \"brandedFare\":\"TIMEFLEXVY\",\n                     \"brandedFareLabel\":\"TIMEFLEX PLUS VUELING\",\n                     \"class\":\"O\",\n                     \"includedCheckedBags\":{\n                        \"quantity\":1\n                     },\n                     \"amenities\":[\n                        {\n                           \"description\":\"SECOND CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"THIRD CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"SNACK OR DRINK\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"MEAL\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"WIFI CONNECTION\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"TRAVEL_SERVICES\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        }\n                     ]\n                  }\n               ]\n            }\n         ]\n      }\n   ],\n   \"dictionaries\":{\n      \"locations\":{\n         \"MXP\":{\n            \"cityCode\":\"MIL\",\n            \"countryCode\":\"IT\"\n         },\n         \"ORY\":{\n            \"cityCode\":\"PAR\",\n            \"countryCode\":\"FR\"\n         }\n      },\n      \"aircraft\":{\n         \"320\":\"AIRBUS A320\",\n         \"321\":\"AIRBUS A321\"\n      },\n      \"currencies\":{\n         \"EUR\":\"EURO\"\n      },\n      \"carriers\":{\n         \"VY\":\"VUELING AIRLINES\",\n         \"IB\":\"IBERIA\"\n      }\n   }\n}")

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
		if len(flightOffer.Itineraries) == 0 ||
			flightOffer.Itineraries == nil ||
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
			departureCity, departureLatitude, departureLongitude, err1 := GetCityFromIATA(flightSegment.Departure.IataCode)
			if err1 != nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}
			destinationCity, destinationLatitude, destinationLongitude, err1 := GetCityFromIATA(flightSegment.Arrival.IataCode)
			if err1 != nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}
			// compute Haversine distance
			distance := internals.ComputeHaversineDistance(departureLatitude, departureLongitude, destinationLatitude, destinationLongitude)
			distances = append(distances, distance)

			segment := model.Segment{
				// segment id is autogenerated
				Departure:   departureCity,
				Destination: destinationCity,
				Date:        parsedTime,
				Duration:    duration,
				Vehicle:     "plane",
				Description: flightSegment.CarrierCode + " " + flightSegment.Number,
				// indicative price set after
				CO2Emitted: internals.ComputeAircraftEmission(int(duration.Hours()), int(duration.Minutes())),
				Distance:   distance,
				NumSegment: numSegment,
				IsOutbound: isOutbound,
				// travel id can't be set here
			}
			flight = append(flight, segment)
		}

		if addToFlights {
			// set indicative price to segments
			totalPrice, err := strconv.ParseFloat(flightOffer.Price.GrandTotal, 64)
			if err != nil {
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

func getCityIATA(s string) (LocationResponse, error) {
	apiUrl := "https://test.api.amadeus.com/v1/reference-data/locations?subType=CITY,AIRPORT&keyword=" + s

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		log.Println("Error while creating the request: ", err)
		return LocationResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error while creating the request: ", err)
		return LocationResponse{}, err
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
			return LocationResponse{}, err
		}

		// repeat request
		var req2 *http.Request
		req2, err = http.NewRequest("GET", apiUrl, nil)
		if err != nil {
			log.Println("Error creating the request:", err)
			return LocationResponse{}, err
		}
		req2.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err = client.Do(req2)
		if err != nil {
			log.Println("Error while creating the request: ", err)
			return LocationResponse{}, err
		}
		defer func() {
			err = resp.Body.Close()
			if err != nil {
				log.Println("Error closing response body:", err)
			}
		}()
		if resp == nil || resp.StatusCode == http.StatusUnauthorized {
			return LocationResponse{}, errors.New("unauthorized")
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error while reading the body: ", err)
		return LocationResponse{}, err
	}

	var locationResp LocationResponse
	err = json.Unmarshal(body, &locationResp)
	if err != nil {
		log.Println("Error parsing JSON: ", err)
		return LocationResponse{}, err
	}

	return locationResp, nil
}

func GetIATAFromCity(city string) (string, error) {
	IATACode := ""

	locationResponse, err := getCityIATA(city)
	if err != nil {
		log.Println("Error getting IATA from city: ", err)
		return "", err
	}

	if locationResponse.Data == nil ||
		len(locationResponse.Data) == 0 {
		return "", fmt.Errorf("no data in the response")
	} else {
		IATACode = locationResponse.Data[0].IataCode
	}

	return IATACode, err
}

func GetCityFromIATA(IATACode string) (string, float64, float64, error) {
	city := ""

	locationResponse, err := getCityIATA(IATACode)
	if err != nil {
		log.Println("Error getting city from IATA: ", err)
		return "", -1, -1, err
	}

	if locationResponse.Data == nil ||
		len(locationResponse.Data) == 0 ||
		locationResponse.Data[0].Address == nil ||
		locationResponse.Data[0].GeoCode == nil {
		return "", -1, -1, fmt.Errorf("no data in the response")
	}

	city = locationResponse.Data[0].Address.CityName
	latitude := locationResponse.Data[0].GeoCode.Latitude
	longitude := locationResponse.Data[0].GeoCode.Longitude

	return city, latitude, longitude, err
}
