package externals

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

type TransitCostResponse struct {
	TransitCost float64 `json:"transit-cost"`
}

func GetTransitCost(from, to, transitMode string, distance int) float64 {
	transitCost := 0.0

	// call api
	apiUrl := "http://localhost:8082/transitcostapi?from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to) + "&mode=" + url.QueryEscape(transitMode) + "&distance=" + strconv.Itoa(distance)
	resp, err := http.Get(apiUrl)
	if err != nil {
		log.Println("Error while getting transit cost from api")
		return transitCost
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body:", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error while reading response body: ", err)
		return transitCost
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		log.Println("Error while getting transit cost from api")
		return transitCost
	}

	var responseTransit TransitCostResponse
	jsonReader := bytes.NewReader(body)
	decoder := json.NewDecoder(jsonReader)
	err = decoder.Decode(&responseTransit)
	if err != nil {
		log.Println("Error while decoding: ", err)
		return transitCost
	}
	transitCost = responseTransit.TransitCost

	return transitCost
}
