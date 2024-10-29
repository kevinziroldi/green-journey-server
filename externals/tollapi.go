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

type TollCostResponse struct {
	TollCost float64 `json:"toll-cost"`
}

func GetTollCost(from, to string, distance int) float64 {
	tollCost := 0.0

	// call api
	apiUrl := "http://localhost:8081/tollapi?from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to) + "&distance=" + url.QueryEscape(strconv.Itoa(distance))
	resp, err := http.Get(apiUrl)
	if err != nil {
		log.Println("Error while getting toll cost from api")
		return tollCost
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
		return tollCost
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		log.Println("Error while getting toll cost from api")
		return tollCost
	}

	var responseToll TollCostResponse
	jsonReader := bytes.NewReader(body)
	decoder := json.NewDecoder(jsonReader)
	err = decoder.Decode(&responseToll)
	if err != nil {
		log.Println("Error while decoding: ", err)
		return tollCost
	}
	tollCost = responseToll.TollCost

	return tollCost
}
