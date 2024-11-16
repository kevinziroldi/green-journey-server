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
	CityName    string  `json:"cityName"`
	CountryName *string `json:"countryName"`
	CountryCode *string `json:"countryCode"`
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

func GetFlights(originName, destinationName string, originLatitude, originLongitude, destinationLatitude, destinationLongitude float64, date time.Time, isOutbound bool) ([][]model.Segment, error) {
	// get cities
	originCity, err := GetCityWithIata(originName, originLatitude, originLongitude)
	if err != nil {
		return nil, err
	}
	destinationCity, err := GetCityWithIata(destinationName, destinationLatitude, destinationLongitude)
	if err != nil {
		return nil, err
	}

	// amadeus flight offers search url
	baseUrl := "https://test.api.amadeus.com/v2/shopping/flight-offers"

	// compute departure-time value
	departureDate := date.Format("2006-01-02")

	// compose url
	params := url.Values{}
	params.Add("originLocationCode", *originCity.Iata)
	params.Add("destinationLocationCode", *destinationCity.Iata)
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

	// check response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	// TODO remove
	//body := []byte("{\n   \"meta\":{\n      \"count\":3,\n      \"links\":{\n         \"self\":\"https://test.api.amadeus.com/v2/shopping/flight-offers?originLocationCode=MXP&destinationLocationCode=PAR&departureDate=2024-10-10&adults=1&max=3\"\n      }\n   },\n   \"data\":[\n      {\n         \"type\":\"flight-offer\",\n         \"id\":\"1\",\n         \"source\":\"GDS\",\n         \"instantTicketingRequired\":false,\n         \"nonHomogeneous\":false,\n         \"oneWay\":false,\n         \"isUpsellOffer\":false,\n         \"lastTicketingDate\":\"2024-10-09\",\n         \"lastTicketingDateTime\":\"2024-10-09\",\n         \"numberOfBookableSeats\":4,\n         \"itineraries\":[\n            {\n               \"duration\":\"PT1H30M\",\n               \"segments\":[\n                  {\n                     \"departure\":{\n                        \"iataCode\":\"MXP\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T09:45:00\"\n                     },\n                     \"arrival\":{\n                        \"iataCode\":\"ORY\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T11:15:00\"\n                     },\n                     \"carrierCode\":\"IB\",\n                     \"number\":\"5775\",\n                     \"aircraft\":{\n                        \"code\":\"320\"\n                     },\n                     \"operating\":{\n                        \"carrierCode\":\"VY\"\n                     },\n                     \"duration\":\"PT1H30M\",\n                     \"id\":\"1\",\n                     \"numberOfStops\":0,\n                     \"blacklistedInEU\":false\n                  }\n               ]\n            }\n         ],\n         \"price\":{\n            \"currency\":\"EUR\",\n            \"total\":\"136.23\",\n            \"base\":\"112.00\",\n            \"fees\":[\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"SUPPLIER\"\n               },\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"TICKETING\"\n               }\n            ],\n            \"grandTotal\":\"136.23\"\n         },\n         \"pricingOptions\":{\n            \"fareType\":[\n               \"PUBLISHED\"\n            ],\n            \"includedCheckedBagsOnly\":true\n         },\n         \"validatingAirlineCodes\":[\n            \"IB\"\n         ],\n         \"travelerPricings\":[\n            {\n               \"travelerId\":\"1\",\n               \"fareOption\":\"STANDARD\",\n               \"travelerType\":\"ADULT\",\n               \"price\":{\n                  \"currency\":\"EUR\",\n                  \"total\":\"136.23\",\n                  \"base\":\"112.00\"\n               },\n               \"fareDetailsBySegment\":[\n                  {\n                     \"segmentId\":\"1\",\n                     \"cabin\":\"ECONOMY\",\n                     \"fareBasis\":\"OOVYTF\",\n                     \"brandedFare\":\"TIMEFLEXVY\",\n                     \"brandedFareLabel\":\"TIMEFLEX PLUS VUELING\",\n                     \"class\":\"O\",\n                     \"includedCheckedBags\":{\n                        \"quantity\":1\n                     },\n                     \"amenities\":[\n                        {\n                           \"description\":\"SECOND CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"THIRD CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"SNACK OR DRINK\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"MEAL\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"WIFI CONNECTION\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"TRAVEL_SERVICES\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        }\n                     ]\n                  }\n               ]\n            }\n         ]\n      },\n      {\n         \"type\":\"flight-offer\",\n         \"id\":\"2\",\n         \"source\":\"GDS\",\n         \"instantTicketingRequired\":false,\n         \"nonHomogeneous\":false,\n         \"oneWay\":false,\n         \"isUpsellOffer\":false,\n         \"lastTicketingDate\":\"2024-10-09\",\n         \"lastTicketingDateTime\":\"2024-10-09\",\n         \"numberOfBookableSeats\":4,\n         \"itineraries\":[\n            {\n               \"duration\":\"PT1H30M\",\n               \"segments\":[\n                  {\n                     \"departure\":{\n                        \"iataCode\":\"MXP\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T14:55:00\"\n                     },\n                     \"arrival\":{\n                        \"iataCode\":\"ORY\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T16:25:00\"\n                     },\n                     \"carrierCode\":\"IB\",\n                     \"number\":\"5779\",\n                     \"aircraft\":{\n                        \"code\":\"321\"\n                     },\n                     \"operating\":{\n                        \"carrierCode\":\"VY\"\n                     },\n                     \"duration\":\"PT1H30M\",\n                     \"id\":\"2\",\n                     \"numberOfStops\":0,\n                     \"blacklistedInEU\":false\n                  }\n               ]\n            }\n         ],\n         \"price\":{\n            \"currency\":\"EUR\",\n            \"total\":\"136.23\",\n            \"base\":\"112.00\",\n            \"fees\":[\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"SUPPLIER\"\n               },\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"TICKETING\"\n               }\n            ],\n            \"grandTotal\":\"136.23\"\n         },\n         \"pricingOptions\":{\n            \"fareType\":[\n               \"PUBLISHED\"\n            ],\n            \"includedCheckedBagsOnly\":true\n         },\n         \"validatingAirlineCodes\":[\n            \"IB\"\n         ],\n         \"travelerPricings\":[\n            {\n               \"travelerId\":\"1\",\n               \"fareOption\":\"STANDARD\",\n               \"travelerType\":\"ADULT\",\n               \"price\":{\n                  \"currency\":\"EUR\",\n                  \"total\":\"136.23\",\n                  \"base\":\"112.00\"\n               },\n               \"fareDetailsBySegment\":[\n                  {\n                     \"segmentId\":\"2\",\n                     \"cabin\":\"ECONOMY\",\n                     \"fareBasis\":\"OOVYTF\",\n                     \"brandedFare\":\"TIMEFLEXVY\",\n                     \"brandedFareLabel\":\"TIMEFLEX PLUS VUELING\",\n                     \"class\":\"O\",\n                     \"includedCheckedBags\":{\n                        \"quantity\":1\n                     },\n                     \"amenities\":[\n                        {\n                           \"description\":\"SECOND CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"THIRD CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"SNACK OR DRINK\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"MEAL\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"WIFI CONNECTION\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"TRAVEL_SERVICES\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        }\n                     ]\n                  }\n               ]\n            }\n         ]\n      },\n      {\n         \"type\":\"flight-offer\",\n         \"id\":\"3\",\n         \"source\":\"GDS\",\n         \"instantTicketingRequired\":false,\n         \"nonHomogeneous\":false,\n         \"oneWay\":false,\n         \"isUpsellOffer\":false,\n         \"lastTicketingDate\":\"2024-10-09\",\n         \"lastTicketingDateTime\":\"2024-10-09\",\n         \"numberOfBookableSeats\":4,\n         \"itineraries\":[\n            {\n               \"duration\":\"PT1H35M\",\n               \"segments\":[\n                  {\n                     \"departure\":{\n                        \"iataCode\":\"MXP\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T20:10:00\"\n                     },\n                     \"arrival\":{\n                        \"iataCode\":\"ORY\",\n                        \"terminal\":\"1\",\n                        \"at\":\"2024-10-10T21:45:00\"\n                     },\n                     \"carrierCode\":\"IB\",\n                     \"number\":\"5777\",\n                     \"aircraft\":{\n                        \"code\":\"321\"\n                     },\n                     \"operating\":{\n                        \"carrierCode\":\"VY\"\n                     },\n                     \"duration\":\"PT1H35M\",\n                     \"id\":\"3\",\n                     \"numberOfStops\":0,\n                     \"blacklistedInEU\":false\n                  }\n               ]\n            }\n         ],\n         \"price\":{\n            \"currency\":\"EUR\",\n            \"total\":\"136.23\",\n            \"base\":\"112.00\",\n            \"fees\":[\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"SUPPLIER\"\n               },\n               {\n                  \"amount\":\"0.00\",\n                  \"type\":\"TICKETING\"\n               }\n            ],\n            \"grandTotal\":\"136.23\"\n         },\n         \"pricingOptions\":{\n            \"fareType\":[\n               \"PUBLISHED\"\n            ],\n            \"includedCheckedBagsOnly\":true\n         },\n         \"validatingAirlineCodes\":[\n            \"IB\"\n         ],\n         \"travelerPricings\":[\n            {\n               \"travelerId\":\"1\",\n               \"fareOption\":\"STANDARD\",\n               \"travelerType\":\"ADULT\",\n               \"price\":{\n                  \"currency\":\"EUR\",\n                  \"total\":\"136.23\",\n                  \"base\":\"112.00\"\n               },\n               \"fareDetailsBySegment\":[\n                  {\n                     \"segmentId\":\"3\",\n                     \"cabin\":\"ECONOMY\",\n                     \"fareBasis\":\"OOVYTF\",\n                     \"brandedFare\":\"TIMEFLEXVY\",\n                     \"brandedFareLabel\":\"TIMEFLEX PLUS VUELING\",\n                     \"class\":\"O\",\n                     \"includedCheckedBags\":{\n                        \"quantity\":1\n                     },\n                     \"amenities\":[\n                        {\n                           \"description\":\"SECOND CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"THIRD CHECKED BAG\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"BAGGAGE\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"SNACK OR DRINK\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"MEAL\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        },\n                        {\n                           \"description\":\"WIFI CONNECTION\",\n                           \"isChargeable\":true,\n                           \"amenityType\":\"TRAVEL_SERVICES\",\n                           \"amenityProvider\":{\n                              \"name\":\"BrandedFare\"\n                           }\n                        }\n                     ]\n                  }\n               ]\n            }\n         ]\n      }\n   ],\n   \"dictionaries\":{\n      \"locations\":{\n         \"MXP\":{\n            \"cityCode\":\"MIL\",\n            \"countryCode\":\"IT\"\n         },\n         \"ORY\":{\n            \"cityCode\":\"PAR\",\n            \"countryCode\":\"FR\"\n         }\n      },\n      \"aircraft\":{\n         \"320\":\"AIRBUS A320\",\n         \"321\":\"AIRBUS A321\"\n      },\n      \"currencies\":{\n         \"EUR\":\"EURO\"\n      },\n      \"carriers\":{\n         \"VY\":\"VUELING AIRLINES\",\n         \"IB\":\"IBERIA\"\n      }\n   }\n}")
	//body := []byte("{\"meta\":{\"count\":3,\"links\":{\"self\":\"https://test.api.amadeus.com/v2/shopping/flight-offers?adults=1&departureDate=2024-11-02&destinationLocationCode=MIL&max=3&originLocationCode=NYC\"}},\"data\":[{\"type\":\"flight-offer\",\"id\":\"1\",\"source\":\"GDS\",\"instantTicketingRequired\":false,\"nonHomogeneous\":false,\"oneWay\":false,\"isUpsellOffer\":false,\"lastTicketingDate\":\"2024-11-01\",\"lastTicketingDateTime\":\"2024-11-01\",\"numberOfBookableSeats\":9,\"itineraries\":[{\"duration\":\"PT29H15M\",\"segments\":[{\"departure\":{\"iataCode\":\"JFK\",\"terminal\":\"4\",\"at\":\"2024-11-02T23:05:00\"},\"arrival\":{\"iataCode\":\"MAD\",\"terminal\":\"1\",\"at\":\"2024-11-03T11:10:00\"},\"carrierCode\":\"UX\",\"number\":\"92\",\"aircraft\":{\"code\":\"788\"},\"operating\":{\"carrierCode\":\"UX\"},\"duration\":\"PT7H5M\",\"id\":\"3\",\"numberOfStops\":0,\"blacklistedInEU\":false},{\"departure\":{\"iataCode\":\"MAD\",\"terminal\":\"2\",\"at\":\"2024-11-04T07:10:00\"},\"arrival\":{\"iataCode\":\"MXP\",\"terminal\":\"1\",\"at\":\"2024-11-04T09:20:00\"},\"carrierCode\":\"UX\",\"number\":\"1065\",\"aircraft\":{\"code\":\"73H\"},\"operating\":{\"carrierCode\":\"UX\"},\"duration\":\"PT2H10M\",\"id\":\"4\",\"numberOfStops\":0,\"blacklistedInEU\":false}]}],\"price\":{\"currency\":\"EUR\",\"total\":\"377.57\",\"base\":\"190.00\",\"fees\":[{\"amount\":\"0.00\",\"type\":\"SUPPLIER\"},{\"amount\":\"0.00\",\"type\":\"TICKETING\"}],\"grandTotal\":\"377.57\",\"additionalServices\":[{\"amount\":\"120.00\",\"type\":\"CHECKED_BAGS\"}]},\"pricingOptions\":{\"fareType\":[\"PUBLISHED\"],\"includedCheckedBagsOnly\":false},\"validatingAirlineCodes\":[\"UX\"],\"travelerPricings\":[{\"travelerId\":\"1\",\"fareOption\":\"STANDARD\",\"travelerType\":\"ADULT\",\"price\":{\"currency\":\"EUR\",\"total\":\"377.57\",\"base\":\"190.00\"},\"fareDetailsBySegment\":[{\"segmentId\":\"3\",\"cabin\":\"ECONOMY\",\"fareBasis\":\"QLYO7L\",\"brandedFare\":\"LITE\",\"brandedFareLabel\":\"LITE\",\"class\":\"Q\",\"includedCheckedBags\":{\"quantity\":0},\"amenities\":[{\"description\":\"FIRST PREPAID BAG\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"PREPAID BAG\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"CARRY ON HAND BAGGAGE\",\"isChargeable\":false,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"PRE RESERVED SEAT ASSIGNMENT\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"PRIORITY BOARDING\",\"isChargeable\":true,\"amenityType\":\"TRAVEL_SERVICES\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"CHANGEABLE TICKET\",\"isChargeable\":true,\"amenityType\":\"BRANDED_FARES\",\"amenityProvider\":{\"name\":\"BrandedFare\"}}]},{\"segmentId\":\"4\",\"cabin\":\"ECONOMY\",\"fareBasis\":\"QLYO7L\",\"brandedFare\":\"LITE\",\"brandedFareLabel\":\"LITE\",\"class\":\"Q\",\"includedCheckedBags\":{\"quantity\":0},\"amenities\":[{\"description\":\"FIRST PREPAID BAG\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"PREPAID BAG\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"CARRY ON HAND BAGGAGE\",\"isChargeable\":false,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"PRE RESERVED SEAT ASSIGNMENT\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"PRIORITY BOARDING\",\"isChargeable\":true,\"amenityType\":\"TRAVEL_SERVICES\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"CHANGEABLE TICKET\",\"isChargeable\":true,\"amenityType\":\"BRANDED_FARES\",\"amenityProvider\":{\"name\":\"BrandedFare\"}}]}]}]},{\"type\":\"flight-offer\",\"id\":\"2\",\"source\":\"GDS\",\"instantTicketingRequired\":false,\"nonHomogeneous\":false,\"oneWay\":false,\"isUpsellOffer\":false,\"lastTicketingDate\":\"2024-11-02\",\"lastTicketingDateTime\":\"2024-11-02\",\"numberOfBookableSeats\":9,\"itineraries\":[{\"duration\":\"PT11H\",\"segments\":[{\"departure\":{\"iataCode\":\"EWR\",\"terminal\":\"B\",\"at\":\"2024-11-02T00:55:00\"},\"arrival\":{\"iataCode\":\"LIS\",\"terminal\":\"1\",\"at\":\"2024-11-02T11:40:00\"},\"carrierCode\":\"TP\",\"number\":\"204\",\"aircraft\":{\"code\":\"32Q\"},\"operating\":{\"carrierCode\":\"TP\"},\"duration\":\"PT6H45M\",\"id\":\"1\",\"numberOfStops\":0,\"blacklistedInEU\":false},{\"departure\":{\"iataCode\":\"LIS\",\"terminal\":\"1\",\"at\":\"2024-11-02T13:10:00\"},\"arrival\":{\"iataCode\":\"MXP\",\"terminal\":\"1\",\"at\":\"2024-11-02T16:55:00\"},\"carrierCode\":\"TP\",\"number\":\"826\",\"aircraft\":{\"code\":\"320\"},\"operating\":{\"carrierCode\":\"TP\"},\"duration\":\"PT2H45M\",\"id\":\"2\",\"numberOfStops\":0,\"blacklistedInEU\":false}]}],\"price\":{\"currency\":\"EUR\",\"total\":\"382.64\",\"base\":\"241.00\",\"fees\":[{\"amount\":\"0.00\",\"type\":\"SUPPLIER\"},{\"amount\":\"0.00\",\"type\":\"TICKETING\"}],\"grandTotal\":\"382.64\",\"additionalServices\":[{\"amount\":\"95.00\",\"type\":\"CHECKED_BAGS\"}]},\"pricingOptions\":{\"fareType\":[\"PUBLISHED\"],\"includedCheckedBagsOnly\":false},\"validatingAirlineCodes\":[\"TP\"],\"travelerPricings\":[{\"travelerId\":\"1\",\"fareOption\":\"STANDARD\",\"travelerType\":\"ADULT\",\"price\":{\"currency\":\"EUR\",\"total\":\"382.64\",\"base\":\"241.00\"},\"fareDetailsBySegment\":[{\"segmentId\":\"1\",\"cabin\":\"ECONOMY\",\"fareBasis\":\"KL0DSI05\",\"brandedFare\":\"DISCOUNT\",\"brandedFareLabel\":\"DISCOUNT\",\"class\":\"K\",\"includedCheckedBags\":{\"quantity\":0},\"amenities\":[{\"description\":\"FIRST BAG UP TO 23KG AND 158CM\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"SECOND BAG UP TO 23KG AND158CM\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"EXTRA LEG ROOM OR FRONT SEAT\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"SEAT RESERVATION\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"MEAL 1\",\"isChargeable\":false,\"amenityType\":\"MEAL\",\"amenityProvider\":{\"name\":\"BrandedFare\"}}]},{\"segmentId\":\"2\",\"cabin\":\"ECONOMY\",\"fareBasis\":\"KL0DSI05\",\"brandedFare\":\"DISCOUNT\",\"brandedFareLabel\":\"DISCOUNT\",\"class\":\"K\",\"includedCheckedBags\":{\"quantity\":0},\"amenities\":[{\"description\":\"FIRST BAG UP TO 23KG AND 158CM\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"SECOND BAG UP TO 23KG AND158CM\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"EXTRA LEG ROOM OR FRONT SEAT\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"SEAT RESERVATION\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"MEAL 1\",\"isChargeable\":false,\"amenityType\":\"MEAL\",\"amenityProvider\":{\"name\":\"BrandedFare\"}}]}]}]},{\"type\":\"flight-offer\",\"id\":\"3\",\"source\":\"GDS\",\"instantTicketingRequired\":false,\"nonHomogeneous\":false,\"oneWay\":false,\"isUpsellOffer\":false,\"lastTicketingDate\":\"2024-11-02\",\"lastTicketingDateTime\":\"2024-11-02\",\"numberOfBookableSeats\":9,\"itineraries\":[{\"duration\":\"PT11H25M\",\"segments\":[{\"departure\":{\"iataCode\":\"EWR\",\"terminal\":\"B\",\"at\":\"2024-11-02T18:40:00\"},\"arrival\":{\"iataCode\":\"LIS\",\"terminal\":\"1\",\"at\":\"2024-11-03T05:25:00\"},\"carrierCode\":\"TP\",\"number\":\"202\",\"aircraft\":{\"code\":\"32Q\"},\"operating\":{\"carrierCode\":\"TP\"},\"duration\":\"PT6H45M\",\"id\":\"5\",\"numberOfStops\":0,\"blacklistedInEU\":false},{\"departure\":{\"iataCode\":\"LIS\",\"terminal\":\"1\",\"at\":\"2024-11-03T07:20:00\"},\"arrival\":{\"iataCode\":\"MXP\",\"terminal\":\"1\",\"at\":\"2024-11-03T11:05:00\"},\"carrierCode\":\"TP\",\"number\":\"822\",\"aircraft\":{\"code\":\"32N\"},\"operating\":{\"carrierCode\":\"TP\"},\"duration\":\"PT2H45M\",\"id\":\"6\",\"numberOfStops\":0,\"blacklistedInEU\":false}]}],\"price\":{\"currency\":\"EUR\",\"total\":\"382.64\",\"base\":\"241.00\",\"fees\":[{\"amount\":\"0.00\",\"type\":\"SUPPLIER\"},{\"amount\":\"0.00\",\"type\":\"TICKETING\"}],\"grandTotal\":\"382.64\",\"additionalServices\":[{\"amount\":\"75.00\",\"type\":\"CHECKED_BAGS\"}]},\"pricingOptions\":{\"fareType\":[\"PUBLISHED\"],\"includedCheckedBagsOnly\":false},\"validatingAirlineCodes\":[\"TP\"],\"travelerPricings\":[{\"travelerId\":\"1\",\"fareOption\":\"STANDARD\",\"travelerType\":\"ADULT\",\"price\":{\"currency\":\"EUR\",\"total\":\"382.64\",\"base\":\"241.00\"},\"fareDetailsBySegment\":[{\"segmentId\":\"5\",\"cabin\":\"ECONOMY\",\"fareBasis\":\"KL0DSI05\",\"brandedFare\":\"DISCOUNT\",\"brandedFareLabel\":\"DISCOUNT\",\"class\":\"K\",\"includedCheckedBags\":{\"quantity\":0},\"amenities\":[{\"description\":\"FIRST BAG UP TO 23KG AND 158CM\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"SECOND BAG UP TO 23KG AND158CM\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"EXTRA LEG ROOM OR FRONT SEAT\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"SEAT RESERVATION\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"MEAL 1\",\"isChargeable\":false,\"amenityType\":\"MEAL\",\"amenityProvider\":{\"name\":\"BrandedFare\"}}]},{\"segmentId\":\"6\",\"cabin\":\"ECONOMY\",\"fareBasis\":\"KL0DSI05\",\"brandedFare\":\"DISCOUNT\",\"brandedFareLabel\":\"DISCOUNT\",\"class\":\"K\",\"includedCheckedBags\":{\"quantity\":0},\"amenities\":[{\"description\":\"FIRST BAG UP TO 23KG AND 158CM\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"SECOND BAG UP TO 23KG AND158CM\",\"isChargeable\":true,\"amenityType\":\"BAGGAGE\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"EXTRA LEG ROOM OR FRONT SEAT\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"SEAT RESERVATION\",\"isChargeable\":true,\"amenityType\":\"PRE_RESERVED_SEAT\",\"amenityProvider\":{\"name\":\"BrandedFare\"}},{\"description\":\"MEAL 1\",\"isChargeable\":false,\"amenityType\":\"MEAL\",\"amenityProvider\":{\"name\":\"BrandedFare\"}}]}]}]}],\"dictionaries\":{\"locations\":{\"EWR\":{\"cityCode\":\"NYC\",\"countryCode\":\"US\"},\"MAD\":{\"cityCode\":\"MAD\",\"countryCode\":\"ES\"},\"MXP\":{\"cityCode\":\"MIL\",\"countryCode\":\"IT\"},\"LIS\":{\"cityCode\":\"LIS\",\"countryCode\":\"PT\"},\"JFK\":{\"cityCode\":\"NYC\",\"countryCode\":\"US\"}},\"aircraft\":{\"320\":\"AIRBUS A320\",\"32Q\":\"AIRBUS A321NEO\",\"788\":\"BOEING 787-8\",\"73H\":\"BOEING 737-800 (WINGLETS)\",\"32N\":\"AIRBUS A320NEO\"},\"currencies\":{\"EUR\":\"EURO\"},\"carriers\":{\"UX\":\"AIR EUROPA\",\"TP\":\"TAP PORTUGAL\"}}}\n")

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
			// fake lat and lon, lower levels don't actually need it
			segmentDepCity, err1 := GetCityFromIATA(flightSegment.Departure.IataCode, 0, 0)
			if err1 != nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}
			// fake lat and lon, lower levels don't actually need it
			segmentDestCity, err1 := GetCityFromIATA(flightSegment.Arrival.IataCode, 0, 0)
			if err1 != nil {
				// the current flight must be discarded
				addToFlights = false
				break
			}
			// compute Haversine distance
			distance := internals.ComputeHaversineDistance(segmentDepCity.Latitude, segmentDepCity.Longitude, segmentDestCity.Latitude, segmentDestCity.Longitude)
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
				Date:               parsedTime,
				Hour:               parsedTime,
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

func GetCityWithIata(name string, latitude, longitude float64) (model.City, error) {
	// this method returns the city_iata for a given city

	cityDAO := db.NewCityDAO(db.GetDB())

	// get by name
	city, err := cityDAO.GetCityByName(name, latitude, longitude)
	if err == nil {
		// check if it has iata
		if city.Iata != nil && *city.Iata != "" {
			return city, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.City{}, err
	}

	// get by coordinates
	deltaCoordinates := 0.2
	city, err = cityDAO.GetCityByCoordinates(latitude, longitude, deltaCoordinates)
	if err == nil {
		// check if it has iata
		if city.Iata != nil && *city.Iata != "" {
			return city, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.City{}, err
	}

	// call api
	err = MakeAirportCityCall(name, latitude, longitude)
	if err != nil {
		log.Println("Error getting IATA from city: ", err)
		return model.City{}, err
	}
	city, err = cityDAO.GetCityByName(name, latitude, longitude)
	if err != nil {
		return model.City{}, err
	}

	return city, nil
}

func GetCityFromIATA(iata string, latitude, longitude float64) (model.City, error) {
	// this method returns the city based on the city_iata or an airport_iata

	cityDAO := db.NewCityDAO(db.GetDB())

	// check existing city_iata
	city, err := cityDAO.GetCityByCityIata(iata)
	if err == nil {
		return city, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.City{}, err
	}
	// check existing airport_iata
	city, err = cityDAO.GetCityByAirportIata(iata)
	if err == nil {
		return city, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.City{}, err
	}

	// else, make an api call
	err = MakeAirportCityCall(iata, latitude, longitude)
	if err != nil {
		return model.City{}, err
	}

	// try getting by city_iata
	city, err = cityDAO.GetCityByCityIata(iata)
	if err == nil {
		return city, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.City{}, err
	}
	// try getting by airport_iata
	city, err = cityDAO.GetCityByAirportIata(iata)
	if err == nil {
		return city, nil
	} else {
		// after api call, the iata is not present in the db
		return model.City{}, err
	}
}

func MakeAirportCityCall(keyword string, latitude, longitude float64) error {
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

	// add cities
	for _, element := range locationResponse.Data {
		if element.Address == nil ||
			element.GeoCode == nil ||
			element.SubType == "" {
			// skip only one result
			continue
		}

		if element.SubType == "CITY" {
			// create city
			cityName := capitalizeFirstLetter(element.Address.CityName)
			countryName := capitalizeFirstLetter(*element.Address.CountryName)

			// customize country name
			if *element.Address.CountryName == "UNITED STATES OF AMERICA" {
				countryName = "United States"
			}

			city := model.City{
				// id autogenerated
				CityName:    cityName,
				CountryName: &countryName,
				CountryCode: element.Address.CountryCode,
				Iata:        element.IataCode,
				Latitude:    element.GeoCode.Latitude,
				Longitude:   element.GeoCode.Longitude,
			}

			// check if city is present
			nameWithoutIata := false
			idCityWithoutIata := -1
			dbCity, err1 := cityDAO.GetCityByName(cityName, latitude, longitude)
			if err1 == nil && dbCity.Iata != nil && *dbCity.Iata != "" {
				// check reasonable distance
				maxDistanceKm := 10.0
				if internals.ComputeHaversineDistance(dbCity.Latitude, dbCity.Longitude, latitude, longitude) < maxDistanceKm {
					// city already present with iata
					break
				}
			} else if err1 == nil && (dbCity.Iata == nil || *dbCity.Iata == "") {
				// city present without iata
				nameWithoutIata = true
				idCityWithoutIata = dbCity.CityID
			}

			// add city to db
			if nameWithoutIata {
				// just update
				fieldsToUpdate := map[string]interface{}{
					"country_name": city.CountryName,
					"country_code": city.CountryCode,
					"city_iata":    city.Iata,
					"latitude":     city.Latitude,
					"longitude":    city.Longitude,
				}
				city, err = cityDAO.UpdateCityById(idCityWithoutIata, fieldsToUpdate)
				if err != nil {
					return err
				}
			} else {
				// create new city
				err = cityDAO.CreateCity(&city)
				if err != nil {
					return err
				}
			}
		}
	}

	// then add airports
	for _, element := range locationResponse.Data {
		if element.Address == nil ||
			element.GeoCode == nil ||
			element.SubType == "" {
			// skip only one airport
			continue
		}

		if element.SubType == "AIRPORT" {
			// check airport not present
			airport, err1 := cityDAO.GetAirportByIata(*element.IataCode)
			if err1 == nil {
				// airport already present
				continue
			}

			// check corresponding city exists in db with same country
			cityName := capitalizeFirstLetter(element.Address.CityName)
			countryName := capitalizeFirstLetter(*element.Address.CountryName)
			countryCode := *element.Address.CountryCode

			// customize country name
			if *element.Address.CountryName == "UNITED STATES OF AMERICA" {
				countryName = "United States"
			}

			dbCity, err1 := cityDAO.GetCityByName(cityName, latitude, longitude)
			if err1 != nil || *dbCity.CountryName != countryName || *dbCity.CountryCode != countryCode {
				// skip only one airport
				continue
			}

			// create airport
			airport = model.Airport{
				// airport id is autogenerated
				AirportName: capitalizeFirstLetter(element.Name),
				AirportIata: *element.IataCode,
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
