package externals

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
)

type FuelCostResponse struct {
	FuelCost float64 `json:"fuel-cost"`
}

func GetFuelCostPerLiter(from string) float64 {
	fuelCostPerLiter := 0.0

	// call api
	apiUrl := "http://localhost:8083/fuelcostapi?location=" + url.QueryEscape(from)
	resp, err := http.Get(apiUrl)
	if err != nil {
		log.Println("Error while creating the request")
		return fuelCostPerLiter
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
		return fuelCostPerLiter
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		log.Println("Error while getting fuel cost from api")
		return fuelCostPerLiter
	}

	var responseFuel FuelCostResponse
	jsonReader := bytes.NewReader(body)
	decoder := json.NewDecoder(jsonReader)
	err = decoder.Decode(&responseFuel)
	if err != nil {
		log.Println("Error while decoding: ", err)
		return fuelCostPerLiter
	}
	fuelCostPerLiter = responseFuel.FuelCost

	return fuelCostPerLiter
}
