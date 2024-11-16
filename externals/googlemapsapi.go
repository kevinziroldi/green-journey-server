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
	Status               string   `json:"status"`
}

type Row struct {
	Elements []Element `json:"elements"`
}

type Element struct {
	Distance *Distance `json:"distance"`
	Duration *Duration `json:"duration"`
	Status   string    `json:"status"`
}

type Distance struct {
	Text  string `json:"text"`
	Value int    `json:"value"`
}

type Duration struct {
	Text  string `json:"text"`
	Value int    `json:"value"`
}

// train directions

type DirectionsResponse struct {
	GeocodedWaypoints []GeocodedWaypoint `json:"geocoded_waypoints"`
	Routes            []Route            `json:"routes"`
	Status            string             `json:"status"`
}

type GeocodedWaypoint struct {
	GeocoderStatus string   `json:"geocoder_status"`
	PlaceID        string   `json:"place_id"`
	Types          []string `json:"types"`
}

type Route struct {
	Legs []Leg `json:"legs"`
}

type Leg struct {
	Distance      *Distance           `json:"distance"`
	Duration      *Duration           `json:"duration"`
	StartAddress  string              `json:"start_address"`
	EndAddress    string              `json:"end_address"`
	StartLocation *GoogleMapsLocation `json:"start_location"`
	EndLocation   *GoogleMapsLocation `json:"end_location"`
	Steps         []Step              `json:"steps"`
}

type Coordinates struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Step struct {
	Distance         *Distance           `json:"distance"`
	Duration         *Duration           `json:"duration"`
	TravelMode       string              `json:"travel_mode"`
	StartLocation    *GoogleMapsLocation `json:"start_location"`
	EndLocation      *GoogleMapsLocation `json:"end_location"`
	Polyline         *Polyline           `json:"polyline"`
	HtmlInstructions string              `json:"html_instructions"`
	TransitDetails   *TransitDetails     `json:"transit_details"`
}

type Polyline struct {
	Points string `json:"points"`
}

type TransitDetails struct {
	ArrivalStop   *Stop        `json:"arrival_stop"`
	DepartureStop *Stop        `json:"departure_stop"`
	ArrivalTime   *Time        `json:"arrival_time"`
	DepartureTime *Time        `json:"departure_time"`
	Headsign      string       `json:"headsign"`
	Line          *TransitLine `json:"line"`
	NumStops      int          `json:"num_stops"`
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
	Text  string `json:"text"`
	Value int64  `json:"value"`
}

type TransitLine struct {
	Name      string   `json:"name"`
	ShortName string   `json:"short_name"`
	Vehicle   *Vehicle `json:"vehicle"`
	Agencies  []Agency `json:"agencies"`
	Color     string   `json:"color"`
	TextColor string   `json:"text_color"`
}

type Vehicle struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Agency struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func InitGoogleMapsApi() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	googleApiKey = os.Getenv("GOOGLE_MAPS_API_KEY")
}

func GetDirectionsBike(originName, destinationName string, originLatitude, originLongitude, destinationLatitude, destinationLongitude float64, date time.Time, hour time.Time, isOutbound bool) ([]model.Segment, error) {
	// get cities (with or without iata)
	originCity, err := GetCityNoIata(originName, originLatitude, originLongitude, false)
	if err != nil {
		return nil, err
	}
	destinationCity, err := GetCityNoIata(destinationName, destinationLatitude, destinationLongitude, false)
	if err != nil {
		return nil, err
	}

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

	segment := model.Segment{
		// id auto increment
		DepartureId:        originCity.CityID,
		DestinationId:      destinationCity.CityID,
		DepartureCity:      originCity.CityName,
		DepartureCountry:   departureCountry,
		DestinationCity:    destinationCity.CityName,
		DestinationCountry: destinationCountry,
		Date:               date,
		Hour:               hour,
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

	return []model.Segment{segment}, nil
}

func GetDirectionsCar(originName, destinationName string, originLatitude, originLongitude, destinationLatitude, destinationLongitude float64, date time.Time, hour time.Time, isOutbound bool) ([]model.Segment, error) {
	// get cities (with or without iata)
	originCity, err := GetCityNoIata(originName, originLatitude, originLongitude, false)
	if err != nil {
		return nil, err
	}
	destinationCity, err := GetCityNoIata(destinationName, destinationLatitude, destinationLongitude, false)
	if err != nil {
		return nil, err
	}

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading the body: ", err)
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	// TODO remove this line and uncomment the api call
	//body := []byte("{\n   \"destination_addresses\" : \n   [\n      \"London, UK\"\n   ],\n   \"origin_addresses\" : \n   [\n      \"Milan, Metropolitan City of Milan, Italy\"\n   ],\n   \"rows\" : \n   [\n      {\n         \"elements\" : \n         [\n            {\n               \"distance\" : \n               {\n                  \"text\" : \"1,196 km\",\n                  \"value\" : 1195939\n               },\n               \"duration\" : \n               {\n                  \"text\" : \"13 hours 41 mins\",\n                  \"value\" : 49241\n               },\n               \"status\" : \"OK\"\n            }\n         ]\n      }\n   ],\n   \"status\" : \"OK\"\n}\n")

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

	segment := model.Segment{
		// id auto increment
		DepartureId:        originCity.CityID,
		DestinationId:      destinationCity.CityID,
		DepartureCity:      originCity.CityName,
		DepartureCountry:   departureCountry,
		DestinationCity:    destinationCity.CityName,
		DestinationCountry: destinationCountry,
		Date:               date,
		Hour:               hour,
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

	return []model.Segment{segment}, nil
}

func GetDirectionsTrain(originName, destinationName string, originLatitude, originLongitude, destinationLatitude, destinationLongitude float64, date, hour time.Time, isOutbound bool) ([]model.Segment, error) {
	// get cities (with or without iata)
	originCity, err := GetCityNoIata(originName, originLatitude, originLongitude, false)
	if err != nil {
		return nil, err
	}
	destinationCity, err := GetCityNoIata(destinationName, destinationLatitude, destinationLongitude, false)
	if err != nil {
		return nil, err
	}

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

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading the body: ", err)
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	// TODO remove this line and uncomment the api call
	//body := []byte("{\n   \"geocoded_waypoints\" : \n   [\n      {\n         \"geocoder_status\" : \"OK\",\n         \"place_id\" : \"ChIJ53USP0nBhkcRjQ50xhPN_zw\",\n         \"types\" : \n         [\n            \"locality\",\n            \"political\"\n         ]\n      },\n      {\n         \"geocoder_status\" : \"OK\",\n         \"place_id\" : \"ChIJD7fiBh9u5kcRYJSMaMOCCwQ\",\n         \"types\" : \n         [\n            \"locality\",\n            \"political\"\n         ]\n      }\n   ],\n   \"routes\" : \n   [\n      {\n         \"bounds\" : \n         {\n            \"northeast\" : \n            {\n               \"lat\" : 48.84494,\n               \"lng\" : 9.2716967\n            },\n            \"southwest\" : \n            {\n               \"lat\" : 45.487137,\n               \"lng\" : 2.37348\n            }\n         },\n         \"copyrights\" : \"Map data ©2024 GeoBasis-DE/BKG (©2009), Google\",\n         \"legs\" : \n         [\n            {\n               \"arrival_time\" : \n               {\n                  \"text\" : \"8:42 AM\",\n                  \"time_zone\" : \"Europe/Paris\",\n                  \"value\" : 1728715320\n               },\n               \"departure_time\" : \n               {\n                  \"text\" : \"8:43 PM\",\n                  \"time_zone\" : \"Europe/Rome\",\n                  \"value\" : 1728672180\n               },\n               \"distance\" : \n               {\n                  \"text\" : \"868 km\",\n                  \"value\" : 867654\n               },\n               \"duration\" : \n               {\n                  \"text\" : \"11 hours 59 mins\",\n                  \"value\" : 43140\n               },\n               \"end_address\" : \"Paris, France\",\n               \"end_location\" : \n               {\n                  \"lat\" : 48.84494,\n                  \"lng\" : 2.37348\n               },\n               \"start_address\" : \"Milan, Metropolitan City of Milan, Italy\",\n               \"start_location\" : \n               {\n                  \"lat\" : 45.487137,\n                  \"lng\" : 9.204822\n               },\n               \"steps\" : \n               [\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"76.2 km\",\n                        \"value\" : 76230\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"1 hour 15 mins\",\n                        \"value\" : 4500\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 46.0049608,\n                        \"lng\" : 8.9464758\n                     },\n                     \"html_instructions\" : \"Train towards Locarno\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"sestGcydw@wIoJoAmACCk@k@QQKK[_@MKu@}@MMWYWYAAo@u@OS[a@ACIMKMW_@MQEEMSMQGIGI[e@QSi@i@IICE]]KKKIUW_@_@aA_ASSy@y@y@y@OOIIIICC]]OMoAqAm@k@[[s@u@c@_@YW_@]]WECUQ_@U]Uc@WGCSISIOG]SsBaAgB{@_@Qy@]_@QUGi@MCAMC_AOWAGA_@A{@Ae@?m@BW@E@c@DA?a@FG@i@NODYHmAl@OHWL[TgAr@sA`A]Ta@Ze@^WPWRWREDw@j@GFw@h@SPyAfAq@`@c@\\\\k@\\\\YNYJm@VQFWJWHSFUDQDa@H_@Dc@FI@mAF_@?UAA?c@A]CEAYCQESCSECAKCQEu@Yc@QYOYOOK_@Sg@_@yAaA[S[Sy@i@WQqFoDSMSMsA{@kAy@}B{A}BoAkAm@iB_AmCyAqCcBmCmBiDcCwEwCiBuAoAaA_EuDcBaBw@u@Y[e@a@YWUUSSk@i@CA][e@e@a@YQQQE[UWS[QiBwAy@m@SMAAWKq@][UIE_@[{@o@[Uu@g@aAm@eAq@u@e@y@k@o@a@WOECYS_Am@GEIGy@i@[U]Ua@YUO[U}@s@YUi@e@c@_@]]CEmBoB_@a@]c@oA{A}A_ByAeBo@q@[]y@{@CCqAyAAA[[cAeASQy@{@w@y@AAsAsAOQMOOM}A_BYY{@aAg@i@gCcCyA{AWYa@a@wAuAMMMMa@a@gAaAw@s@a@_@iCeCeAeAu@u@oCsC{AeByAcBcDmDmGaHmHyHeAcAwHuHwHeIoKyK[]wC{CWW[]wAyAk@k@MOwAyA]]aFgFwD{Dy@{@g@g@QQUWUW{AyAkBmBe@e@y@{@c@c@c@c@y@{@y@}@eDuDCEiCeDEE{AwAk@m@EE_A}@gAcAOOOMsCqC][yAsA][]]][eA_AqAgAu@m@[WgByAo@i@aEyD{@m@{K{H}@m@eAo@OKQMaAk@aBaAe@YoEoCqDoBkAo@cF_Dk@_@MEyEmC{CiBmB_AmAo@kCm@u@?u@?Q?aADa@HEBy@l@A@{@r@_@Zi@b@mAfA}@v@{@p@yFhEo@jAiApBWd@ABk@fA?@o@lAMTINc@z@g@h@gApBWd@Wd@Wd@iApBWd@Wd@y@xA[h@c@x@aBtC_@r@OVEJaAdBiApBWd@o@jAWd@i@~@KPS\\\\KPuAbCkDfGgAjBU`@qAzB}@`BWd@iApBgAnBmCzEkIfOwDzGyBzDWd@_BvCWd@}@`BcClEaBvCy@zA}@`BgErHaErHSZaBvCiAnBS\\\\S\\\\Wd@q@jAwEvI{@`BsDzGq@jA_AdBGJINMTCDk@dAU`@[h@iExHi@~@gAnBmAtB[j@INeAdBo@jASZmDnGc@p@sBtDwCrFkAxBIH{@zAoBjDkArBEFy@tAGHq@hAs@fAs@fAW^yAlBuCtC{ArAsAhAwBlAiAl@GBe@Xw@Tk@Ri@NOHOJmBp@g@P}@\\\\eA\\\\SHSF_A\\\\C@a@Ns@XODcDlAqAd@[LWHsC`AaBl@eBl@qAf@oAd@wAf@y@XqA`@uAh@wAh@iBj@m@VwAf@sAf@gA^iA`@}@Zi@RQFQHa@LyBv@IBa@NmBr@}CfAkC`AgA`@oBr@cBl@yDrA_EtAeRxG}Bz@gAb@a@P}@b@e@XKFYPcAr@A?m@d@ONIFIF[XOLONWTSPa@`@CB{@~@}@jAWXa@l@a@l@W`@?@o@bACFKTYb@INKTMT[j@S^o@jAWb@g@~@uAfCQZqAzBwAjCeAlB_@t@MV{@dBM\\\\INYp@gArCm@fB]bAQn@g@`Bu@vC[vAKf@Mt@}@tEQjA_@nCEh@MhAStBQxBOnBM`BGdAE`@G|@SxCGr@KpAY~DKnAEn@YtDWpDOvBU|Cs@rIIhAIdAMxAEh@MpBSdDAFS|BGr@UjDQ~Bc@~Fg@`Gi@zHEv@O~AKdBQ~BYxD{@tLe@fGk@pHe@bGk@`Ia@fFk@jIa@dFw@|KK|AYrDu@xJKfB[bEMlBk@nFa@|DCRGZG`@]tBQfAGViAhFAFaAlDGPGROj@o@lBs@pBqAlDEJ}@lBiBjDGHITaA|AINiAdByFbHoBlB_BlAiBrA_BjA]RIDKDuAv@a@TSJcBx@m@X_@PaBt@eClAkBv@oCrAwB`AOHqB~@u@\\\\eClAiEpB_FxBuFjC_EjBeF`CwE|BcEjBgEpBMDuBbAGBIDQHa@P_@PeGrC_F|BsB`AsCtAkExBsCfBuCpB}BlB}@v@GDUTm@l@yAxAg@j@o@r@uBnCa@f@gCvDsBbD{AtCs@rAoAdCiBtDcArBA@g@~@Sd@GNKRKR_@n@Wd@e@v@IRUf@Sd@_B|CKPcDpGKRy@bBk@fAo@pAi@fA{BnEs@xAEJcAnB_CrEaApBiBtDeCnEcBlCs@dAU\\\\m@t@_@d@c@j@gAlAGHa@^QN{@x@GH{@t@g@`@YTOJa@VSNcC~A}BnAwB~@wChAoD`AM@a@Hc@Fc@H_@FA?c@Fc@FK@WBc@Bc@Bi@D]@c@BG?Y?gA@gA@]?i@AuAEaCUm@E_BYe@G_@IgAQQEs@Q{Aa@m@Sa@Oa@OgE_BkCo@E?eAQc@I]Eg@Gc@CQCyACA?a@@gABkABeDZoCp@qEbBqEpC_FnEuDvCgExBmFzAyCZyANsEXw@HmCZa@LgBh@{Ab@a@TKFMFeAj@aAf@}C|B]V{A~As@|@MNMP_@b@o@bAaBlCCBm@nAo@rAA@g@pAi@rAg@tAi@rAi@rAg@rAaAfCABk@z@s@fA_AvAgCvCUTwBlBA?_@V_BbAk@\\\\ULcB|@iBn@mDv@}A\\\\G@gAFE@aABK?K@k@@[AgAAi@A_CUmASiASmDu@GA}@SiB_@AAcAUeAUSEwBe@iBa@e@IaH}AkGoAmCm@w@QyCo@c@Ka@KYGWGUGc@Ic@Ia@Ic@KeAS_B[}Dg@iBImCIwAG_CJsDPC?eBVmAPSBSBcAZu@TOFoCfAuDrBaBvAWVuDtDmAjAsHvHq@t@iApA}@j@}AlAsAdB{C|D{BzEaCrE_ApA]f@sBjB_BdAkAv@wAl@wB~@iE`AgBPm@CsBNy@@_AC}@GoAOe@I_ASa@KaAa@_@Qm@[WQa@[[Ya@]a@a@W]qRgWiDuESUuBmCg@q@GGYa@]c@w@sA_@}@Yo@Ui@M]y@}BaAkC_AaCQ_@cCiHc@uAmBaFy@eCSk@K][w@Qi@Qa@Ws@c@iAWu@Sk@Si@Sk@?A{@}BSk@CIQ[Ye@GMaA_BCC{@m@_@WgAu@YK_@QIEYCc@Em@G]C_@Ba@@YBm@HMBUFa@Ne@N]NIDIDo@f@w@r@KJYPk@^KL]\\\\_@^{ArBMPg@|@m@pAYv@Sh@Yt@w@rBEHoAnDc@fAg@rAKXKZS`@Sr@e@tAKXeAvCoAfDw@rBaB|CCBmAbB]b@kCfD{@dAgBzBu@bAMNMN[`@MNMP]b@EDUZY`@gArACDc@h@iAtAqB|BIHmApA}@`ACB{@|@gBrBm@r@e@j@gCtC]^aBxAaCxB_EvDMJcC~BiDhEu@`AA?kAdBu@xAABW`@Wb@Yd@QXQVq@hAGJeA|A_DxE_F|HCBu@bAu@dAw@|@[\\\\WXED[\\\\uA|A]\\\\[\\\\]^A@gAhAKLQRILSXc@f@[^gBtBQZCDKTUh@u@vBw@vBqApDmAbD_AfCuBvFMZqB|EsBvEwBnFUf@w@hBkBxDO\\\\sCxFe@`A{AdD_AtBUf@{@rBCFqAhDi@vAg@pAe@nAm@vA_A~BQf@oAjDCFINkAhCcAvBe@vA?@Qj@CHK`@CHSv@I^Sj@CHYrAGZGTI`@On@CNI\\\\Mh@Ml@GVe@tB?@]zAc@nBEPCJEPCPENYrAGTWr@CHK^Mf@e@~Aq@lBw@vBe@vAq@|BGPg@zAc@vA[fAYhACH]fBi@hBWz@_@jAIVCNMb@e@zA{@lCq@pBmAvCs@bBGJ{@~As@lAMVOVe@z@g@~@qAzBeAlBkAvBcBzCMROTiAtB[l@i@dAcAhB[n@e@~@w@zAGHINQXu@tAQ\\\\gApBeAlBaDxFuAdCaAjBEFy@hBKRKRMR]n@QXq@jAo@jAm@dAUb@QZEJUf@k@lA_@x@IXSx@YtAI^a@bBYlAMr@Q^Uj@a@v@A@Yb@MPg@v@U`@Wb@mEbHYb@Y`@eBpCMT[f@e@~@e@~@Qd@[~@UjAI\\\\EVEZGn@CTADA\\\\E|@AlA?`@@n@@j@@XFx@L|@Lt@DTFVDTVt@Rl@Zt@h@~@b@t@fA~A\\\\f@R`@Vh@Tj@^fA`@bBNr@Hp@Hr@N`B@|@@j@?X?hBAfAGzBGnAGt@AVKv@SvAO`AQt@[hAYz@y@bCgAvCgBvEWv@Sr@Op@Mj@Gj@Gj@ElACz@AbAAlABvA?`AAv@Et@C`@Gh@K|@Kj@YhAQn@_@`A]p@]j@CBW\\\\ILaAbAi@^q@^u@V}@To@Du@Fa@Ce@AoBSg@Gs@Im@Ca@Bq@Dk@La@JYJo@Xu@h@YVIFSP[Xq@p@KJ]ZONeBdB]\\\\sAtAqBrBo@r@{A|A_Az@yAxAw@x@o@l@[\\\\{AxAA@y@v@_@^}@j@q@Vg@Nw@Pw@D_A?mAMc@I]I_A[a@Uq@k@YUk@u@EGeAcBCCiAmB[k@k@mAAAg@{@w@sAQWq@u@kAoACCqBkBYWYYc@Yi@i@w@s@OSe@i@kA}AuAiBa@i@uAkBgAyAkAgBu@gAe@s@q@eACEe@k@{@kA[a@_AcAs@i@WOSMMIq@[o@SgA[s@Km@GwAIq@Cy@Cw@CsDIaABg@@cBH]D_@DmCXcAJkALuC^uDZWBy@HsAZC?uAb@o@V_@PiAl@q@^[P[PyAr@iBl@A?{AZG@iATcBZmBl@G@c@Ps@^C@YPcAr@A@o@h@MJ[\\\\eAdAQP[\\\\A?mAlAIH{@z@mBnBk@n@e@f@i@h@YZ}@z@CBw@t@o@p@IHuAvAsAfAwAx@}Bx@eAL]DkBBi@Eq@EaAOsAc@k@U_@Ou@c@YM]QKGmAq@o@]k@[cB}@aAg@cAi@cGgDa@UeEyBsBgAOIMIYO[QaCoA{BoAGAmD}A{B}@e@Qe@SA?_Ae@cCgBu@i@uBqA_CaBGE{@]EC_ASICy@Ke@AQ?iA@gFCm@?iAKUEc@Iy@Wk@[iG_DUKq@S_AOA?mBKmCKm@C{BOI?[?qBIsDQG?w@?w@B{@LI@y@VoAn@gAp@OJOJaF|C_Al@wA~@GBGDaBdA_@T_Al@_@T_@VaAj@gBhAOJOH_Al@aAl@eC|AkC`Bi@Xu@`@wHtCeA^iB|@_Bl@{CjAc@Nk@Za@ZOHq@h@QTSR_@d@o@~@Q\\\\yB|D{EtI_@p@c@p@SXIJOR]d@A@OPON]\\\\]X]REBWL[N_@Ls@ReAVeCj@a@L]JWJIDOHq@^QNQNc@`@g@l@ILoAfBKNi@r@KPeA|AgA`BaC~BMJgCnBkBv@a@Pa@PmDz@uCZoBCUC]CWC]GWEiBa@sC{@e@OgA[oAYc@Ku@MC?c@GiAKmCI_@AG?mAHcAL}A\\\\{@Xg@Pa@PeAj@]V[Tu@p@m@j@UXk@t@[b@Wf@a@v@Yn@Qb@Ob@Qj@IZK^IZOp@I`@Kp@Ih@MnAGn@AVCb@UlFCb@Cp@Ex@EhBIrBG~ACb@Cj@KvCAZCd@AVCp@AP?HAVCX?RAVCVAVC\\\\Eb@ANARAPCZ?DG`@Ip@Ib@o@pDMr@Kn@Or@CRGXMx@SfAOv@Kl@K`@Kf@Ql@Ql@ABIRUh@MVS`@U^OTMP]b@[XUTMJQNIFYPMFKHMFKDKDKDKByCp@UFc@Je@Ni@JWFYDa@Fa@BUBU@M?a@?e@?UAUAOA]E[Eu@K]Ei@EYGYEYCe@GWA[CQ@O?O@gBN]BY@]@[@YAY?W?[AUAe@C]Ae@C[C_@EWGAAUEUIQG]KUK_@MYKSIUIOEME]IOEYG[EI?SC]Aa@CU?WAMAM?QAUAQCUCOCUESEMEMEKCSIUIMGMEMGSIQEOI[KWKMCMGQEOISGMGQI[KOGSISIYMSKQMOIUQQMQMQOQMUOOMQMSOSOYUWQSMUQWSWSSOUUSSMOQSSWSYMQKQKOOUMSMQIMGGKOMQ[][Yc@_@g@]c@WYOSMWOk@]w@g@YQYUk@g@c@e@}@}@]]]YSQWSQK]Sg@Yo@Y]OYKYGs@Oc@Gc@E_@AO?g@?U@c@Be@F}ATi@J{@N}@Nk@Hi@Fi@Bq@?I?s@Ae@?A?kBBcB?sAAaDHaE`@YH{DhAQJwAv@g@d@yAtAg@d@m@v@u@~@[b@CDi@`AGNQ\\\\EHUf@ADQf@Sf@ADQd@Md@K`@Od@?@Kd@Md@CNCNG^ADGb@Ed@Eh@C`@AVAZAv@?~@@d@@`@@HBj@BTBb@D\\\\F^?@F\\\\DXNv@FVJl@BHLn@N`A@DDZ?DD`@D`@Bl@Bb@@z@?f@Cj@Cd@C\\\\Gp@Gd@CTI`@Ib@IVK\\\\M^KZUh@KROX]f@g@n@[ZOLSN[RA@WLk@TA@a@Lc@HUBE?e@BE@e@?e@Ea@Ea@I[K[MYMMGKGi@[YSk@a@g@][SSMm@[a@OMGm@Og@Mi@I[AEAs@CiACmBGyACa@Cq@COAYCCAWECAUE]IUK[Ma@SYSYUIKGE]_@[c@U_@U_@MWOa@KWK]Oe@GSOo@WeA?CIWGYEOIYK[Mc@]y@m@kAA?Ya@i@o@u@m@k@]o@WIEc@Ke@KUA}@?[@YBYDIBUFQFC?_@L}Ad@_AVu@To@PiCj@@F\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 45.487137,\n                        \"lng\" : 9.204822\n                     },\n                     \"transit_details\" : \n                     {\n                        \"arrival_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 46.0049608,\n                              \"lng\" : 8.9464758\n                           },\n                           \"name\" : \"Lugano\"\n                        },\n                        \"arrival_time\" : \n                        {\n                           \"text\" : \"9:58 PM\",\n                           \"time_zone\" : \"Europe/Zurich\",\n                           \"value\" : 1728676680\n                        },\n                        \"departure_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 45.487137,\n                              \"lng\" : 9.204822\n                           },\n                           \"name\" : \"Milano Centrale Railway Station\"\n                        },\n                        \"departure_time\" : \n                        {\n                           \"text\" : \"8:43 PM\",\n                           \"time_zone\" : \"Europe/Rome\",\n                           \"value\" : 1728672180\n                        },\n                        \"headsign\" : \"Locarno\",\n                        \"line\" : \n                        {\n                           \"agencies\" : \n                           [\n                              {\n                                 \"name\" : \"Schweizerische Bundesbahnen\",\n                                 \"phone\" : \"011 41 848 446 688\",\n                                 \"url\" : \"http://www.sbb.ch/en/timetable\"\n                              }\n                           ],\n                           \"color\" : \"#ec0000\",\n                           \"name\" : \"Locarno - Chiasso - Como - Milano\",\n                           \"short_name\" : \"RE80\",\n                           \"text_color\" : \"#ffffff\",\n                           \"url\" : \"https://www.trenord.it/linee-e-orari/circolazione/le-nostre-linee/locarno--%20chiasso%20-%20como%20-%20milano/?code=RE80\",\n                           \"vehicle\" : \n                           {\n                              \"icon\" : \"//maps.gstatic.com/mapfiles/transit/iw2/6/ch-zurich-train.png\",\n                              \"name\" : \"Train\",\n                              \"type\" : \"HEAVY_RAIL\"\n                           }\n                        },\n                        \"num_stops\" : 8\n                     },\n                     \"travel_mode\" : \"TRANSIT\"\n                  },\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"0.2 km\",\n                        \"value\" : 172\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"3 mins\",\n                        \"value\" : 172\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 46.0057891,\n                        \"lng\" : 8.9464209\n                     },\n                     \"html_instructions\" : \"Walk to Lugano\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"qexwGwkru@sHr@\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 46.00425,\n                        \"lng\" : 8.9466786\n                     },\n                     \"steps\" : \n                     [\n                        {\n                           \"distance\" : \n                           {\n                              \"text\" : \"0.2 km\",\n                              \"value\" : 172\n                           },\n                           \"duration\" : \n                           {\n                              \"text\" : \"3 mins\",\n                              \"value\" : 172\n                           },\n                           \"end_location\" : \n                           {\n                              \"lat\" : 46.0057891,\n                              \"lng\" : 8.9464209\n                           },\n                           \"polyline\" : \n                           {\n                              \"points\" : \"qexwGwkru@sHr@\"\n                           },\n                           \"start_location\" : \n                           {\n                              \"lat\" : 46.00425,\n                              \"lng\" : 8.9466786\n                           },\n                           \"travel_mode\" : \"WALKING\"\n                        }\n                     ],\n                     \"travel_mode\" : \"WALKING\"\n                  },\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"255 km\",\n                        \"value\" : 254968\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"3 hours 0 mins\",\n                        \"value\" : 10800\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 47.5463462,\n                        \"lng\" : 7.591359699999999\n                     },\n                     \"html_instructions\" : \"Long distance train towards Basel SBB\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"eoxwGcjru@AOOBmAT}Bd@YFE@e@Jm@La@HcB`@QBC@WLe@TULSLMLIDKHi@f@KNSTKLU`@S^Wd@MZIT[hAMj@Mp@Gb@w@zFo@`DOd@a@fAaBdDaD`E[^eDdE}EfGg@n@iAnAqBzBsBzBwA~Aa@p@QRQPIHe@b@KHKHk@\\\\c@Rg@N}@Lo@DY?S?[EWCOCMCKAc@IWGu@OiAUk@MWEUIc@OA?QGOGSK]Q]Ow@c@OIMGEAc@UWM_@Sc@Sc@Qi@QOEOCQGWEUEEAWC]Cu@GA?gAI{@Ea@AQAs@Ga@CkBGyAG{@CaDa@yGgA{LgBcGwBwLkIc`@c_@s}@_c@mvAoPoOkCqL}DiYkNq|@el@cg@w[qcAa}@i}@ke@szAm`@{nAmRuXb@_s@dQeh@tOk@NkCZ{@J{@JgEf@o@Hq@FaBFgB@}BK}AS_AQo@Ok@Q}B_AeBaAm@_@m@a@{AoA_BaBgB}By@uAcBeDs@_Bs@aBoG_OKSISsDmImAsCw@gBw@eBuAqCoA}ByA_CCCuAyBw@qAe@{@KOm@gAOYw@_BUa@Uc@iAsB_@w@MWaBwCoAiCcCqEyAsCa@s@m@iAmCiFKSqAkCcJeQ}A{CGIEIkAoBS[qAeBQUQUEEeAuAKMKMu@cAa@g@o@{@mBeCiAyAMQOQMSmBeCmAaBy@eAw@cAiA}AMMSWSWw@aAw@aAmEuFaBwBqAcBcB_A}BoAsBkAOIQKcCuAaCwAc@Wu@{@y@_A[]IKsCmCgBuBMOi@s@y@iASWSYo@}@e@o@k@w@c@k@cAuA[a@kCcDaEeFW]aDkD_@i@{DwECCoBiCSWEGSU[e@[_@w@eAw@_A_@g@k@y@UWe@k@s@q@aBuAi@i@qB}AoD_CwBwAuA_AIEyA{@_Ag@WMkBgAa@Qu@[qA]OEsCo@KAc@IIA_AKi@Co@CyA?_@Bc@BM?aAPyBf@m@LcCh@_@Ha@J{A^cATE@[F_AR[F}@LaAJ]?I?i@Gs@KAAm@Oi@Om@Ww@_@kA{@wA{@oAw@}@c@]OOIo@Uk@Sk@O]G_@G_@CW@y@?a@@G?uCJU@}CFgAFqFJwBBuBHiBDsAH{AL[BaCNi@@mBF}CLiBJ_AFY@eAFe@@eABc@@A?}AHK@_AF}@Hs@LUDgAV{Ab@kAb@OHgAd@}A|@[PkAz@UP{BfB?@qAtAEDoBlBED{AxA}AvAu@v@w@x@A@{EtEQPiCbCKJsCrCA@eBfBgBdBON_BxAWTgAjAe@d@{@x@[\\\\e@h@u@`Ae@p@{@tAW`@OXmAbCQ\\\\CHk@pAi@tA?@u@|BmAtD_AtCKXEHCHoAvDe@vAkDlJ{@jBm@dAMT_ArASZEDuA~A[^k@p@cAhA}ArAMJq@d@i@`@UL_@T]PeAh@SJo@Ra@N{@X}A`@_ANiAR}F\\\\_GZyKj@A?c@BmCFO@M@M?a@DkBJc@Bc@Bc@DeDRoFZW@c@BgAFcBJE?Q@Q@qAFgAFoH^iFXgAHsCPeBRuAPmDhAmBz@oBlAkAbAIHw@r@{@bAaBjBm@p@gBrBe@h@}BhCiEnEURwCfC}JtIOLaG|EaBbAwBhAgAj@mAl@q@^aDlBcAj@cFpCkJbF[PmItEaB~@eAl@aExBgBx@{DpAyCz@UHuC`Aa@N}Al@yChAWJ}Br@mA\\\\mC|@OFEB_Bv@iAj@_CjAMDgDzA{C`By@b@qDpBuAn@}@b@c@XqBrAcC`AYLoClAm@V[JcC|@}ChAkC`AmAd@KDKDoBp@w@Xe@Ry@f@A@kAz@q@f@KHw@t@s@v@[\\\\eArA{AlB}EhGORi@d@iB`BsAt@u@b@oAd@s@XyA\\\\_@HUDg@FG?YDiAF{@@S?g@Ck@CSC[?YEM?u@M_BQ_@E_@Eq@GW?g@?Y?W?O@_@Bm@Hc@HM@q@Nc@Hi@T}Al@yAj@oBx@y@ZcDxAo@XQHiEfBuAl@kBv@}Ap@cDxAgBt@c@P[LaAZI@_@JqAV}@J_@DC?_@Bm@Dc@@aA@_@?C?k@AsAMMAyCWqHm@E?c@CI?gCLcAHc@Dk@F{Dh@QDy@VyHhDC@aCbAgBt@MFMDw@ZyAn@EBaA\\\\]Hk@NqBd@qJlB_BZwGvAyCh@s@LsAXaB^uB`@qAVsB`@}Cn@gATy@Po@LKBIBeATkCj@qB`@C@w@P_Dn@iDr@wCl@eDp@{Dx@sCj@q@Ps@Xk@ZURKFYVKLq@z@e@v@Wl@s@pB[zA?@On@Mj@G^Kf@EVi@nCKf@]fBi@nC[zAAFMj@WnACNMn@g@nCMn@Ox@YvAi@nCMn@[~AMl@a@pBGXGTETGT_@xBEVOz@Gf@ETEVGn@MbAE`@Ip@E\\\\MbA?@E^C^AHMdBIfBErAM`GATCp@?DQhC[rCKr@a@zBCNEPCROlCIbBEt@GxCO`EEt@e@fIs@dFQfAUfAkAjFs@xDUjAKh@]dBGX{@|Ck@tB}@vCaAbDcApCQj@Sj@Od@[z@y@bCQj@CHsAxC[p@g@fAyAbDAByAzCa@x@wAdCq@nAcBnCu@lAoEvGeBjCABgBnC{AdCsBxCOTaBlCe@v@_A|AyA|Bq@~@YXyRxKqUtMqW~NsW~N_GfDsPrJqYrPs[~Qc\\\\hRgTzMa@TcYhQqbEdhCgyDzxBwtAfw@kvCztAivC|tAmeBd}@GBa@ReqBjdAiv@f_@gu@p^c@TwuDttByuDvtBi{@hg@i{@jg@{aGbhCyaGlhCibGlnCmsAvm@_@R{mAtj@yOhHwNvGyOhHwNvGmGtCkF`CaMxFkHlDA?mHnDeBr@cA`@wKjEa@Pa@Pm@TcAb@a@PMDSJ}An@[JeAZaMtDwC|@qJpCgBh@aMpDg@NeCx@SHuGdBkBd@cARMDm@PmC~@_B`@eBh@cBd@sCr@sBh@qA\\\\oAb@y@f@G@{Dz@uGxAmLjCqFjAi@L{IdByD|@MBKD[FuAZGBMB{@R_@HgB`@yPvDoG|A}BhAaCj@C?qDz@_H~A{G|AmAVeARiB^SDQB_B\\\\iB\\\\eL|BgATeARMBODeATeAVeBb@yG|AiN`DgATuBb@wHbB[H}@TG@uAVg@LE@gBf@G@[FwGzAeATsEbAa@Jc@Ha@J{A\\\\[FC@]HcBb@g@HaAReAVC?_@Fg@F_AJuANW?c@?U@eB@aGCmA?}EAqDCC?c@Ck@Ei@K{@S]Go@QiAWu@]i@Ko@IQAa@Eu@CQ@_@?aBJM@sDXa@Bi@JcAX_@JSFYNYPo@^i@f@g@h@a@l@e@r@WZm@x@e@j@[XURUNe@X[La@NgBp@a@Ji@NYD[FI?_@Fg@@S@g@?g@EGAQEg@K]Ie@WIGMIg@]g@e@]_@_DuD}@o@_@YYQSIs@MMAcACM?e@E_DDoFR{FR{FR{FRc@BuENgADwEPgADwEPgAByFTgABoLb@kBFmL`@oCJc@BeJZsDLc@Bif@bBkBHc@@yGVg@Dw@HiCVcAL}Bv@qAp@qAp@e@NODQBu@JG?UBA?o@Aq@Gk@G[Ci@IgAOcBWgC]o@GQAw@@q@HSD_@H_@Ne@TWNiChB}@l@OJg@T}@ZUCSE]A{@Ci@AkD?eCHyBRO@sCp@_FhBe@PODyB|@_Bn@eBr@IBkChAyGnCYLuBt@oDnAeBp@cDnAiGzBwGjCkAd@eA`@qCfA}Bx@uAh@eA`@sBx@wBd@kCTo@Bg@Dq@DqDEuBCgC?aBL}Cd@i@RMF]L_A`@}@BaCHK?kBCK?_AI_AUmCu@eAYSG_AWSGAKyA[e@KaASs@Oc@G[Cc@GiBQoCEc@AQ?iAFsBXgDr@k@LyAd@uBfAm@Z}Az@kAn@w@Zo@TMHULi@ZgAr@gBlAUPSP_A|@w@fAW\\\\MPQPw@t@g@X_@TcAd@I@WFYFG@I?}@Be@?e@?m@AoAMKCi@Iw@UEAgA]}@m@_@UoA_AECGE_Au@yCgCQMKM[YMMQSs@y@cD{CUUGG]]}CsCoBmB}C}CgBcB]]uCoCSSe@e@}OgOqEiEiJyIyAuA_B{AUUa@_@}@y@QOKK][_@[KIeDeDkHcHwBuB[]][qAoAaC{BGGkAcAyCaCeAs@c@WACaAm@MCAAcFqDqB_BqDsCwCwB_@WoAk@gA]eA[qAYcAQg@KyAMiBC}ALiCl@kBv@uBrAiCbCKJwBtBSXORU^Yf@[n@e@hAADe@|Ac@`Bk@~BAD}@tDGVENERKZo@fBSd@EH}@zAU\\\\a@j@c@f@c@b@u@f@kAr@gAd@{B~@{Ad@k@TeC|@aBv@qAx@uAdAiAdAs@x@mAzAOPu@bA{@hAW^mAfBY^sBtCqBlCcBxB}BzCaExFmBlC_FtGa@j@o@z@kA|ACDUZQVmBfCeCdD]`@OToFnHiB`CKNaBtBw@bAu@bAQVSVu@bAu@bAu@bAW^Yd@Q^Qf@GPK\\\\Sv@Mh@Kp@Iz@Gx@Ex@?n@@p@@t@`@`LRlF@^B\\\\Hr@Jx@Nr@Rv@JVHZVl@tAfDRh@Pn@Nl@Lr@L|@Ht@F~@@r@@n@Av@C|@IhAQrDE|@Aj@Ar@Ap@@|@Dx@Br@BXBRz@rKh@|DP`AZhA~@~Cr@zBpApEd@tCNrC@rB?DCdB?BW~CSfAER_@tACDg@xAITKTCLe@x@m@x@ILONKHOLqAv@QL_Ah@o@`@mEbC]V_@ZUTGD[^QXW`@OXO\\\\Ob@Q`@Qp@On@Kj@ABK|@I|@GlA_@`MClA?dADdAHdALhAXpAV|@Xx@b@z@j@`AbEfGjAhB`@l@|@~Av@vAb@jAZhAXvAVvAN`BHbAHrAAfAAnCWdDAZ?BAzAMhICrCO~CKzDQrE@d@FjBOvHAVEhC?RGxC?x@GfGA`ACzA?@EvBAXA`@A`@?FC`BA\\\\A\\\\K|CK~C[jFGx@Ev@K|AEhAA\\\\AhA?Z?r@@X?JBf@@NBd@Dt@Bp@Dr@?N@b@B~@BvACpAEhAE`@CXAFG^Mt@Or@U~@WnAELI`@Ox@SvAIt@Gn@?@QbBSpCOrB?BWbDO`CSjCCZEh@IlAMxAIl@CVGXKh@I^Ol@Sp@Wj@S^[p@EJMVINQ\\\\EHWf@A@Sh@O^KZ[`A]nA?@GXAVCVCXCt@Ar@?hB?l@EnB?VKfBKx@EPOz@_@pAc@zAs@hCc@vAQp@s@hCg@hBo@tB[`Ae@lAe@fA[h@QZy@vA]j@e@l@g@d@A@i@`@mAn@g@Ve@Pk@Ti@V_@Pi@^YVWRW\\\\c@j@wAvBaB`CIJ{@nAm@r@YZe@\\\\]Rg@Ne@Po@Ti@Lu@Xk@Z_@Ta@Xg@h@m@v@EHW^Wj@Yf@S`@CDOZYh@i@x@a@h@KJWVg@^i@Xq@XE@c@Jg@Ho@D_AF{@DA?g@Di@FWFIBKBMDc@Rg@ZEBo@f@c@b@[`@W`@S\\\\GLGJq@tAo@xAILYf@g@x@c@n@y@jAWZk@r@k@x@a@r@i@bAi@nAoAbDoAdDiA`DIPa@hAg@|Aa@nAGPe@`B[dAa@rAIRWx@w@xBkAfDeAtCa@fAc@jAu@pB_@hAWv@a@pAQl@Qf@AF}@dDy@rCCR[`ACD[v@[d@a@h@}@bAw@x@k@j@e@j@]f@_@t@EH[t@[~@Oh@Qx@Mx@QxAABWfBWtAMf@Of@Sl@GJMVQ\\\\a@p@}@rAcAvAiA~Ac@r@o@hAm@rAc@lA]fA[lAa@pAe@bAg@x@c@h@Y\\\\]^g@d@w@t@_AbAk@t@ORc@t@a@r@IPINUf@u@lBIRi@bBq@|Bu@jCY~@c@nAi@jASb@O\\\\u@tAYd@Wb@q@~@kA|AiB`C_BtB_@f@W\\\\_AlAs@~@SXSVUZ_@j@CB]b@c@l@{BzCuBfDsA`CgBzCsBlD[h@c@t@s@jAABu@rAKLo@t@{A`BKLSR[XKJONg@f@_@`@QVKNOZYb@Yp@Wt@Sn@Qp@Oz@Mv@Gn@Cf@EtA@~A@d@?BBh@Dn@Db@Fj@Lt@TlAb@vBd@vBJb@P|@\\\\~ABNXtA\\\\`B^bBV|ADRLj@FTPj@Pb@Pd@P`@FL`@l@Z`@X^\\\\Zh@^^Pp@Xz@V@@z@Xl@Tj@ZZNTTFDh@h@|@jA@Bh@`AhAxBPd@BFBDv@lBj@zADNd@~APh@Vv@Tx@V|@t@`CZbA^fAb@dBNr@Pp@VtAXnBPbAJj@RfALh@BHFTJ\\\\J\\\\Pb@JTNZ`@f@X^jAtAb@f@NNl@l@@@jAlAPRTVFFHJb@d@~@`At@z@x@dAr@t@\\\\^RPDDd@\\\\r@j@dAl@z@h@l@^v@h@v@l@~@l@XPZPDBdBdAnAbAPNvApAvBnBjBdBfAdAlAhAHHHHNN|A~AhAdBbCrD@B~@fAJLZ^rA`Bd@h@`AlAFJt@tAP^p@lATd@Vd@^p@f@`Ab@z@^~@FRnCfIr@hBnAhC~@rBt@zBjB|FBHj@|Az@nBlBhER`@d@`BHRLr@ZzA?Bd@rCfA|GBJ|AvJF^F\\\\Jl@r@fC@D`@dAf@hAl@jAzApB|AjB|BlCFHJL@BJNl@z@`@l@bCfDdBxBJL@BPTpBbCZ`@XZn@p@l@j@RRLH|AjA`BlALHn@f@VRpAfANNl@n@Z^h@l@^f@v@hAn@fA\\\\l@f@bATb@|@fBPZ`@n@TVFFNPd@l@fA~@t@p@bBzA|@z@nAhAPNNLZX~@|@RRZ`@PXLVNTRb@L\\\\Z|@j@rBt@rCTv@XdA@FjAhEZjA`@|ARp@Rr@BLBJx@tCRn@FLZbAJXn@~A|@rBv@fBVd@P`@HNdA~BP\\\\`@|@d@fA`AtBr@tAv@zAPZLXFL@BlAnChCrF~@rB\\\\t@`@z@bBrDd@bAJRrAzCp@xAt@|ApAvCz@jBvAzC@@@DPXNXNZlAvBtBrDfAlB^l@LT`@p@Xf@|B~DTb@P^Xl@Vl@^hAr@`DfAjFvBnKFb@R`BLvAPjD?BXxHLzCLlD?VA^An@Ez@El@ANIv@Gn@Ij@Ib@I`@Kf@c@pA_@|@IPQZQXYb@g@h@]Z]ZGFWTqD~CMJYXWT]Z_Ar@YXSRQPKH{CtCKL]`@ORw@dAgAlBaBlCuA|BWd@EHYd@]t@s@fBMXKVa@x@k@~@cA|AKLQVa@Xe@X]RkAf@WHIDeBr@a@PcA`@e@PUH[LIDYN{@`@y@\\\\}@^_DpAe@RKHE@a@Xg@ZQPWVUTWZIJIL]f@Ub@Ub@?@KTUl@Qb@Oj@IZCJQt@Oz@ERAJKx@OrACRCTOxA[xCUxBO|AQpB[jCYlCEX[zCU`CQ|AOxAC\\\\Q~A]fDKjAEd@ALAPAHC~@CtACjACp@CZAXGl@Ij@E\\\\ETGTIZGTM`@Un@Yn@SZWd@MTc@r@U`@CDCDa@|@GNGTGXg@dCSnAOdAUvAOjAUdBi@bDUrAYfBSrAe@dDW`BIj@SzAIr@_ApHU|A[hCKt@If@m@xD_@zCQpAWhBQ~CIrD?@AtAAv@@j@@~@LzAd@zBx@bCv@nA~DfF~IvJVXvApB@@n@pAp@bCJj@XzBXlED`@?@NtALp@Hb@Pn@Rl@JZHPHRJPNVfAnANP@@DDNJFDJFJFTNZLh@RZFN@N?l@Bj@AP?D@^AF?b@IhCm@^SbAe@^SbAg@NINGh@YzAu@zAs@xAu@RInC{A`B{@TMhAo@h@Yp@_@PINGXM@A@Ax@c@v@c@JENIRIVQr@i@BAJKr@u@`@i@T[X_@NSDIJUVm@Vo@To@Nq@He@\\\\cCD{@Di@@g@BuA?q@CcBGaBCSEq@AOGi@Ge@Gi@Ki@EYMk@Mi@Oc@CICGWi@IUGIMUe@s@e@g@YWk@a@KGKGGG_@QCCo@SOEo@OSIQEOGQACAi@EGAUCUA]Eu@Ck@@_AHcAHm@Fw@FeAHkBPyCZxC[jBQdAIv@Gl@GbAI~@Ij@At@B\\\\DT@TBF@h@DB@P@NFPDRHn@NNDn@RBB^PFFJFJFj@`@XVd@f@d@r@LTFHHTVh@BFBHNb@Lh@Lj@DXJh@Fh@Fd@Fh@@NDp@BRF`BBbB?p@CtAAf@Eh@Ez@]bCId@Op@Un@Wn@Wl@KTEHORY^UZa@h@s@t@KJC@s@h@WPSHOHKDw@b@y@b@A@A@YLOFQHq@^i@XiAn@ULaBz@oCzASHyAt@{Ar@{At@i@XOFOHcAf@_@RcAd@_@RiCl@c@HOFSFUHMFOFEBIDGDCB]VSPUP[Z]b@GHMTYd@Q^]~@CDUz@Ov@Kn@KbAUpCIhAIjACZSfCCXCVEb@MvBGt@QbBAJo@tCu@xBq@tAs@jAST}@~@_Al@}@b@KFMFwAZ{BTsA@ULuADmB?y@DcBJ}ADyAJeANyA\\\\q@VkAj@a@Zc@XiBtAw@h@_A`@}A\\\\mALC?O?O@uEMM?[AiAGC?a@?a@?E?yAIWAW?yADQBs@HQDe@Hw@\\\\IDiBfAWZW\\\\gAxAwAxBGJiAjB]j@_@l@o@dAYd@GJOV{@vAw@pAs@lAMPQVq@dAMTORq@hAMRy@nASVSVKLWVc@^YT[ROJ[NQHA?i@Ts@Ps@Jo@FW@_@?U?[Ak@GcAMuAUeAQi@KqAUQCSESEcBWkAUA?eB[eBYk@KaBY}AW[GmB]g@Ke@IQGSEu@Uy@Y_A_@g@Ww@c@m@_@SMQMMKOKQMUQQOWUQOQSUSa@c@e@e@m@q@a@c@GI[[QSUSSQOMOKQMQMQKSIQIQGQGQGMCAASESCWEOAYCK?QAQ@U?M?M@UBK@K@MBMBODUHUHWJWJKFQLOH[RMLSRQL[^QPU\\\\MRA@U^MTWj@O^KRQj@Mb@Ol@GZEPE`@EZCNCNGl@EXC^CTAVAPAPAVAX?l@?X?V@V@b@@ZBZBZBX?@Dd@Hh@DVBRF\\\\Jb@HXFVFRFPJZJTJVJTFLZl@PZR\\\\NVb@v@Xf@l@fAVb@HLXh@T^Vb@Zj@R\\\\^n@Xf@Zh@^r@@@R^Xd@Zj@@@Vd@Vb@Zj@\\\\l@NXJR^l@R^V`@Xj@HNV`@Xj@Vb@DFXd@`@t@Xd@Xh@R\\\\T`@Vf@`@p@R\\\\P\\\\Xf@h@~@Vd@DFb@v@P\\\\NXRb@Xn@N^L\\\\HZPh@Nf@Pr@Jh@BHJj@N`AL|@JjAFv@Dz@DlA@v@?N@v@AhACn@A`@Cj@Cr@Gv@Iv@Ir@In@Ox@S`ASx@Qr@Od@Qf@]x@Yp@w@`Bm@lA_AjB_AjBe@bA]p@i@dAc@|@q@rAg@dAk@hAo@lAm@lAq@tAKRIPg@`Aq@tAs@vACDeAvBy@bBiA~BeArBo@pAw@`BOXa@x@q@tAw@xAi@x@i@t@g@h@_@\\\\]Xw@h@o@\\\\o@X{@TG@[Fe@Fi@Fa@@m@?UAs@Ea@Ey@K}@Oo@K_@G{AUsB]wB]eB[_@G_@Gq@Ik@Ei@Cg@Aa@@{@Dy@Fw@Jq@Ng@Lo@Ry@^m@X[Ru@d@o@d@IHMLk@h@s@t@]b@e@l@c@p@q@`AgAbBm@|@s@dA_AtAw@jAu@fA_AtAe@r@Yb@U^o@dAWf@A?Yn@Sd@Wn@Yv@Qd@St@YfA[zAWzAQtAO|AM~AKvBIdBAXUpESfEc@zJUxEQbES|DI|AKrAOnACPW~Ac@tBe@pB_AxDcA~D}@rD}@nDgArEyA|F{ApG_AxDKb@oAbFwAzFwAbGOn@m@|Bc@lBsAvFgAlEi@dCShAWjBCPIv@OvBCr@GxACz@ErAKrD?@Q|EMdEEvAMxDEzAEdBM|DCbACj@W`IWhIWxHAf@Cr@GhBMpD?^A^Q|FEbBSjGI~BUhEg@jHQ`BSpBYbCMx@CRCRs@xECLKn@CNoAhGqAfFk@zBGNe@vAGLENUr@Qd@c@rAWl@cB~De@dAgAxBsAfC{@zAo@jAsA|BqA|BgBxCuBzDs@xAy@`BGNGNuAjDSh@k@~AaA~Ci@bBy@vC]dA{@rCg@fBuArEm@jBs@~BITSz@GPo@rB}@zCm@nBg@`BkB~FwA`EcAvCuAfDaBxDEJyA~C{@`BoA~BqBtDqDzFa@n@u@hAg@p@u@bAi@t@W^a@j@m@z@w@fA_@f@iA|Am@x@}AvBsBrCsBnCgA|A_@f@g@r@{@jA}@nAmAfBk@z@u@fAiAnBgAlBoAjCqBfE{A`DcBpDQ\\\\EHCH}AbDi@jAkCvFq@tASb@iAbCGLGLQ\\\\iAbC{AbDWh@c@~@eBpDc@`AaDzGa@~@c@|@yBtEi@jAsApCoA`CEFmApBc@n@o@|@oA`BwB`CyB~Bi@j@EFkAlA_AbAm@n@IHGFUVSRcDlDa@b@oBrBwAzAUXc@b@yGhHIH_@`@{@`Aa@`@uAxAeAfAcChCw@x@kBlBMNONw@|@MNm@l@[\\\\]^}AdBWXEDsAtAy@|@wC|CqAtA{@~@sBxB[`@c@h@GHMVoBnC]h@iB`CaAhAsApAcA|@}B|BsAlAYVuBpB_C|BuBrByAzAkA~@u@n@OHk@j@i@\\\\OJSLOFWJiAf@uB|@WJmEhBgAh@y@d@cAr@}@x@_@b@]^o@z@e@r@aAjBeAdCeAnDCJ]dBYbBYhCU|DOtBY~EEd@Er@?He@lHmAnRg@vHUrDEp@Ep@uA~TC^Gz@GbA[jEIrA_@hGgBpYEf@KvBcAhPe@pHc@zGAZInAUtDoDzj@IlAWtDCj@Ej@Y|Eo@zKW`EGfAYdEq@dK_@rFUjDIvAEb@Cd@Gn@ChAC`@CZK|AIvAMhBu@~KKdBYnFMtCEpBGbDCxA?hB?pA?j@DxDDzCDpBLpCX|D@`@@JDb@LnAFbAf@~FvApQNvBJbCFhC@LBbANrEBhABlBDdD?zF@~I?pE@`G?f@Af@@|E@bR?z@?dA@~D?N?B@xR@t@Drk@?t@Dl^?|C?hB?l@?j@@lD?n@ABCjAG~AK`AOxAW|AUdAWdA]bAa@`Ac@`A[h@o@bAEDc@j@c@f@o@f@i@`@w@d@_@P_A^aBh@_Cp@aFzAsF~AeAZuA\\\\oC|@eGfByF`BeCx@UFSFUFWFsFbBkDdAmDdAoEtAgBh@gBh@_@HOFgBh@KDe@LaAZ_Cr@iB`@m@J}@NeADi@@C?c@@c@?c@?c@Aa@CYCMAUAkBUQCu@OgAYs@OyBu@aBk@wCeAoDuAKEkDuAAAsAg@}@]WKWKiAa@mBq@{By@gBo@_A]SIaFkBaGyBoHmCi@UOG[Ku@]q@Sa@K_@I_@Gg@Kq@Im@Eg@EoA?}@FiAN_ATK@o@TgAb@cBv@yB~@uAn@gEjBe@RcBt@uCnAqClAmBz@kBv@aGjCyB`AwB~@cAd@m@XyBbAe@Re@PeBv@eCfAcAb@yDfBiLdFQHaBv@wAr@qB|@aCbAwB`Au@ZgBx@ID}BbA_@NIBa@RA?oB|@gAh@s@TGBiBz@kDtAOFMDcAb@GB{@^iAh@cBt@yAn@iAd@yAn@YN[N_CfA]P{D`BqB|@iAf@u@^aGbC_Bt@}@`@{@b@{Al@}Ar@uB~@gAf@iBv@sAl@{An@cGjCgAf@{OfHiElBMFMDoBz@mAh@qCjA_@NsClA}D~A_Bv@SHa@PwAl@o@XgAd@YJC@KDKFuCnAuLjF}I|D}VzKiElBQFOHcBt@UJIBIDUJA@{An@aAb@wLjFkEjBgDzA}An@_ElBq@^_Bv@sAp@KFA?IF}@f@e@\\\\qLjIu@h@KHuBnAIDOJMHIDeBdA_An@i@\\\\wCtBGFGFi@h@[ZUVuBvBoDjDON_A`A}CzCw@v@sHpHqBnBwBvBQPQPqApAsDpDoEnEiCfCcEbEoClCyAvAm@l@MLSRSR_C~BSRUTwCtCqElEqArACBuCtCgHjHaEbEwA~AKL]`@cEpFiBhCsd@dn@qEhGiCjDW\\\\MPi@t@{BtCyD`FqFjHsAnBu@dAENSXg@l@e@l@u@~@c@j@IJg@l@QPc@Xq@\\\\[LYHWDm@LY@]Bc@Ac@E_@EEAkA]eFcBaA[oHyBoAa@q@QwEyA}FiB}DmAg@Oa@KQGcCw@yHaCc@M_IcCcDaA?AcBc@kAOaAOiAGyAEqBDgAHaAHu@NeAPYDsBf@a@JIBgEbAw@PwA^_@L_Bd@KDQFgBh@g@NaANK@gBFkBK}@IoAMuBUa@EIAK?aASkBWUIaAWaAWcD]aFg@}@KoCY}@SMEA?e@Q{@]s@c@q@i@WU[]e@i@SYc@q@c@u@Yo@c@aASi@q@}Am@wAACc@gASg@mAwCgAqCgAkBaBoCmAqBcAyA_BqBu@e@gDmBSKGEOE{Ak@cASkASy@E_AAq@DW@s@Hk@J{A`@OBMDYF}A^qBj@{@RqEjAI@IBoBh@eAXeAVeAXIBG@wFzA{@TkD`AcKrCiE`Aw@T}Cz@uD`AoEhAsBl@iB`@E@E@gD|@mA^c@JUFgBf@sGfBcWtGiFtAyKtC_@HcEfAeAXaFpAsCt@y@RuBh@cCn@aDz@eLvCuUfGkCx@gAZkEdAyIzBed@bLc@JsFxAWF}KlCUFiJ~Bc@JgBd@[HyJhCqO~DgTpFUF_LxC]J}EjAwQrEkAZ[HgWvGm@FwNtDc@LE@cIdCiCp@{Cx@sA^{CdAyBlAgClBgA|@uBpB}BlB_FxEwC|C[`@]`@iAfBk@fA]v@cB~DgCpGKVKVo@jAsAvBkA~Ao@z@uArAqAhA{FpFgA`A}@z@y@v@wDnD{BxBgAnAc@l@cAzAOXQZeAvBOX[t@qArD?@YjAm@`CIb@Id@a@bCk@fDq@pDY`Bc@nBK^c@|AADc@pAKVIVe@nAUj@k@rACFk@dAS^qAtBKLKNy@bAUXe@`@qAnAeCpBUPcGtEmA~@WNiAbAo@r@STMNKNe@r@MTWh@MVMXs@nBGTITWbAMd@Kd@WrAETKv@WxCIrAEhC@lBH~AJrA@H\\\\rBr@vFD\\\\DZ\\\\nCFf@BTDV\\\\vCh@jEBTDTNnAXzBfB|Np@|FX|BJr@BTBTJnB?DDr@B`@@R@f@BrA?r@At@A`@SfIGhE?lA?\\\\?P@`@@HBj@@^Hx@J~@DXBXXdBf@tBJ^Xx@f@vA@Bj@nAJRJRh@x@d@j@LLvAzAvAzAvAzAZb@@B\\\\h@N\\\\\\\\r@R^Xp@FNTr@f@fBVpAPx@N`ANlADp@LpB?RBp@?h@C`CAh@?ZEbAGx@I`AGh@]lCqAvLYpCE`@E^i@jEeAlIi@dEu@xEiAfHIf@eBnKi@fDI\\\\G\\\\]zAU~@y@pCq@pBMb@_AnCUt@]hAS`AUhAQdAOjASlBOtBK|AUhDs@`LUjD_@tF]zFW~DIjA[tEe@tHCZu@zLqArSMrB?HSzCC^CRQ|CQlCOxB_@|F_@fGa@dGe@zH_@vF_@~F_@vF]`GUdDIjAY~E[xEKnBEx@C~@ClA?jAA~AB|C?BD`DBfDAdA?nBA~BAx@Cr@Cr@G|@KnACNOpA[zBY|AQr@On@Ql@ELUr@Uj@qAjDgB~EKL_BlEaAlCIR]z@s@pBo@hBe@lAkAdDsAnDgAzCy@zBIPiGzPgBxEUr@Wp@{CfIWp@Qf@}EzMGPGN{CdJ{B`GiAdC}EjKEFMVCFIP_@p@KPMRmBlDOXQ^uBrDGJeAfBw@pAk@z@sBjDeBnCeD|EkAdB{@tA}C~DyAnBIJKJeBrBKLKN{EnGmBfCKLKLiBnC_DtEe@t@wAfCeDzFug@zjAWh@a~AfpDUh@iNp[{BbF_F|Hw@pAoAzAwCnDGHqDfDyDbDgIjGiBlAy@j@_Ar@k@f@s@n@m@j@w@x@w@|@q@z@k@t@g@r@[f@U\\\\c@r@QX}@`Bq@tAu@`Be@lAe@lAo@jBCHe@xAe@`Bc@|Aa@bBYhA{@nDw@hDQv@i@|B]rAi@zBo@tCi@xBI\\\\q@xB_@jAe@`BiA`FgAxEaAtE]vAQt@Qn@c@|AAHa@bBo@rB[`Ac@hBiBdIy@lDyCpMIX{@vDsA|Fm@hCYjAABWjAOj@]nAk@nBw@lCMb@Sh@Wr@e@zAe@dBA@k@pBW|@Qn@e@hB_@~AYnAG^Gb@E^CJEn@E|@AXAf@?j@?@AjBCl@Cf@APCXGv@ETMjAEXEf@QzAIj@Il@Kj@CNQp@Or@On@A@o@lCc@jBAD_@dBId@EPO`ASdAKr@Kj@Gf@Kz@CXCXC`@Ab@Ah@Aj@?n@@Z?L@d@Dp@Bd@Fj@NjA?BJh@FZF^Hf@VtANdAh@zDFh@Fn@Dn@Bp@@h@?`@?TAd@A\\\\Cp@ElAGxACf@Cf@KfAUfCIx@MrAMpAIn@In@Id@O~@GZKr@Kh@Ih@EXETGj@gAtLgAvLKfAADEl@Cj@Cp@CdACl@Et@Ep@MdBCh@I|@Gn@Gd@I`@G`@I`@Of@e@tAa@jAK^GXI\\\\Sx@Sx@Qt@Il@Kl@I`@GVGVk@lC_@|AMh@Qt@q@lC_@~ABB\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 46.0057907,\n                        \"lng\" : 8.946415199999999\n                     },\n                     \"transit_details\" : \n                     {\n                        \"arrival_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 47.5463462,\n                              \"lng\" : 7.591359699999999\n                           },\n                           \"name\" : \"Basel SBB\"\n                        },\n                        \"arrival_time\" : \n                        {\n                           \"text\" : \"1:02 AM\",\n                           \"time_zone\" : \"Europe/Zurich\",\n                           \"value\" : 1728687720\n                        },\n                        \"departure_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 46.0057907,\n                              \"lng\" : 8.946415199999999\n                           },\n                           \"name\" : \"Lugano\"\n                        },\n                        \"departure_time\" : \n                        {\n                           \"text\" : \"10:02 PM\",\n                           \"time_zone\" : \"Europe/Zurich\",\n                           \"value\" : 1728676920\n                        },\n                        \"headsign\" : \"Basel SBB\",\n                        \"line\" : \n                        {\n                           \"agencies\" : \n                           [\n                              {\n                                 \"name\" : \"Schweizerische Bundesbahnen\",\n                                 \"phone\" : \"011 41 848 446 688\",\n                                 \"url\" : \"http://www.sbb.ch/en/timetable\"\n                              }\n                           ],\n                           \"color\" : \"#ec0000\",\n                           \"short_name\" : \"IC21\",\n                           \"text_color\" : \"#ffffff\",\n                           \"vehicle\" : \n                           {\n                              \"icon\" : \"//maps.gstatic.com/mapfiles/transit/iw2/6/ch-zurich-train.png\",\n                              \"name\" : \"Long distance train\",\n                              \"type\" : \"LONG_DISTANCE_TRAIN\"\n                           }\n                        },\n                        \"num_stops\" : 7\n                     },\n                     \"travel_mode\" : \"TRANSIT\"\n                  },\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"0.4 km\",\n                        \"value\" : 431\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"7 mins\",\n                        \"value\" : 394\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 47.5462406,\n                        \"lng\" : 7.592135600000001\n                     },\n                     \"html_instructions\" : \"Walk to Basel SBB\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"okeaH{tim@}BtJQr@]SPo@?AReAHY@GjC{K\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 47.5463242,\n                        \"lng\" : 7.591343499999999\n                     },\n                     \"steps\" : \n                     [\n                        {\n                           \"building_level\" : \n                           {\n                              \"number\" : 0\n                           },\n                           \"distance\" : \n                           {\n                              \"text\" : \"0.2 km\",\n                              \"value\" : 159\n                           },\n                           \"duration\" : \n                           {\n                              \"text\" : \"2 mins\",\n                              \"value\" : 128\n                           },\n                           \"end_location\" : \n                           {\n                              \"lat\" : 47.5469545,\n                              \"lng\" : 7.5894658\n                           },\n                           \"html_instructions\" : \"Walk for 160m\",\n                           \"polyline\" : \n                           {\n                              \"points\" : \"okeaH{tim@}BtJ\"\n                           },\n                           \"start_location\" : \n                           {\n                              \"lat\" : 47.5463242,\n                              \"lng\" : 7.591343499999999\n                           },\n                           \"travel_mode\" : \"WALKING\"\n                        },\n                        {\n                           \"building_level\" : \n                           {\n                              \"number\" : 0\n                           },\n                           \"distance\" : \n                           {\n                              \"text\" : \"19 m\",\n                              \"value\" : 19\n                           },\n                           \"duration\" : \n                           {\n                              \"text\" : \"1 min\",\n                              \"value\" : 32\n                           },\n                           \"end_location\" : \n                           {\n                              \"lat\" : 47.5470425,\n                              \"lng\" : 7.5892091\n                           },\n                           \"html_instructions\" : \"Take the escalator \\u003cb\\u003eup\\u003c/b\\u003e to Obergeschoss 1\",\n                           \"polyline\" : \n                           {\n                              \"points\" : \"moeaHeiim@Qr@\"\n                           },\n                           \"start_location\" : \n                           {\n                              \"lat\" : 47.5469545,\n                              \"lng\" : 7.5894658\n                           },\n                           \"travel_mode\" : \"WALKING\"\n                        },\n                        {\n                           \"building_level\" : \n                           {\n                              \"number\" : 1\n                           },\n                           \"distance\" : \n                           {\n                              \"text\" : \"18 m\",\n                              \"value\" : 18\n                           },\n                           \"duration\" : \n                           {\n                              \"text\" : \"1 min\",\n                              \"value\" : 28\n                           },\n                           \"end_location\" : \n                           {\n                              \"lat\" : 47.5471864,\n                              \"lng\" : 7.589307999999999\n                           },\n                           \"html_instructions\" : \"Walk for 18m\",\n                           \"polyline\" : \n                           {\n                              \"points\" : \"_peaHqgim@]S\"\n                           },\n                           \"start_location\" : \n                           {\n                              \"lat\" : 47.5470425,\n                              \"lng\" : 7.5892091\n                           },\n                           \"travel_mode\" : \"WALKING\"\n                        },\n                        {\n                           \"building_level\" : \n                           {\n                              \"number\" : 1\n                           },\n                           \"distance\" : \n                           {\n                              \"text\" : \"19 m\",\n                              \"value\" : 19\n                           },\n                           \"duration\" : \n                           {\n                              \"text\" : \"1 min\",\n                              \"value\" : 31\n                           },\n                           \"end_location\" : \n                           {\n                              \"lat\" : 47.5471034,\n                              \"lng\" : 7.5895527\n                           },\n                           \"html_instructions\" : \"Take the escalator \\u003cb\\u003edown\\u003c/b\\u003e to Erdgeschoss\",\n                           \"polyline\" : \n                           {\n                              \"points\" : \"}peaHehim@Po@\"\n                           },\n                           \"start_location\" : \n                           {\n                              \"lat\" : 47.5471864,\n                              \"lng\" : 7.589307999999999\n                           },\n                           \"travel_mode\" : \"WALKING\"\n                        },\n                        {\n                           \"building_level\" : \n                           {\n                              \"number\" : 0\n                           },\n                           \"distance\" : \n                           {\n                              \"text\" : \"0.2 km\",\n                              \"value\" : 216\n                           },\n                           \"duration\" : \n                           {\n                              \"text\" : \"3 mins\",\n                              \"value\" : 175\n                           },\n                           \"end_location\" : \n                           {\n                              \"lat\" : 47.5462406,\n                              \"lng\" : 7.592135600000001\n                           },\n                           \"html_instructions\" : \"Walk for 220m\",\n                           \"polyline\" : \n                           {\n                              \"points\" : \"kpeaHuiim@?AReAHY@GjC{K\"\n                           },\n                           \"start_location\" : \n                           {\n                              \"lat\" : 47.5471034,\n                              \"lng\" : 7.5895527\n                           },\n                           \"travel_mode\" : \"WALKING\"\n                        }\n                     ],\n                     \"travel_mode\" : \"WALKING\"\n                  },\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"38.0 km\",\n                        \"value\" : 38039\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"39 mins\",\n                        \"value\" : 2340\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 47.3619469,\n                        \"lng\" : 7.350569699999999\n                     },\n                     \"html_instructions\" : \"Commuter train towards Delémont\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"wkeaHkzim@IG@Er@}CJa@@Eb@kBJ[La@JYHYL]DMBGJ[FKDMFSHUDKNg@Pe@JYFOPg@Pg@Pg@`@kAd@uAb@iAJ]J[JYb@oAVgADKDOH_@BIJc@Po@HSL_@HSv@oBZy@Ti@h@wARi@l@_BzCwH^_A\\\\y@Tk@Te@LYNWh@_AZg@bBiCp@cAnBmDfBsC\\\\c@h@s@tAwBn@cA\\\\i@|BkDh@s@TWZ]ZW\\\\Y`@Wf@Yx@_@ZMf@O`@Ip@MPCRE^EXG^Gd@IXEbAOvAUl@Gl@IPC|@OlAQhDe@NClBYNCLAdBW^EdAQXCXEl@Ip@IHAZEv@MTCLCd@Kn@Mb@K\\\\KVIVI\\\\M^Op@YbBs@`CcAx@_@hAg@`Aa@LGlAi@t@]t@[@Aj@U\\\\O^QRIj@W`@QLGpB}@fAc@l@Yd@S`@Qd@Sl@WBAbAe@^QlB{@l@S@C^OVIRGLCd@QPIXIPCp@Sf@K^GVA\\\\AT?V@`@BVDTFVFb@L~@^~B|@zAl@tAj@b@PPHNDp@X`A\\\\RHf@P~@^`DnAbBp@vBz@dAb@hBr@nEbB^PtAj@zAl@r@V~An@b@R`A\\\\XJVJlBt@nAf@FBvCfAj@TtAj@j@RjBv@nBv@dC`AfAb@b@Nz@Vl@NTD`@F|@J`@Bp@?Z?XAd@Af@Eh@IVEXG~@Wv@U^Kh@Oh@Qr@Gp@Il@Az@?j@Dh@Fb@F`@J^JtCdAlA`@XH`@Hr@FH@l@BxBMJ@pCMTA~BMtDSp@CbBILAdDQr@Cn@Cn@EpBM^Aj@CnAGvAIx@Er@CVAzCMvBE|A@rAJz@LJ@lB^VHPDRFJBrBv@j@Rh@PxAh@fA`@^JfBr@~Bx@lBn@pBt@zAh@nA\\\\hAb@bA^fA^rAf@d@Pn@TpAh@hAl@r@f@pAbAjC|Bj@d@f@b@\\\\^LJfA|@vAlAnA|@p@d@l@`@JHTNp@b@dAl@hBt@hA\\\\bCf@tBXB?t@DbAD`ADbAJz@@vAE^A|@QLC^Gj@Wp@i@BCX[V]BCl@}@FQBGHSDKDKTk@L]Tk@J[Z{@X{@b@eAj@sAJW`BcDz@cB~AuCXa@FIh@s@V[XSVQj@]LG|@]~@Mp@CD?\\\\@b@BD?p@Hb@Lr@\\\\XRB@TLd@d@z@`AT`@Xd@Xd@Vb@z@xANXFJJRbA`BXb@r@hA~AfCvGdLb@x@rA`CdAfBb@r@^p@f@x@b@t@b@n@TRRVd@`@h@ZVNz@b@p@Pj@LPBTFV@f@?l@A`@Gl@I~@SBC`@QNIzAs@~BmAnDiBtAq@tAg@v@Q|@GlAAl@BrAXb@Pr@\\\\b@XLJLHZZv@~@RZh@z@d@|@n@jAtAjCDHXl@V`ABHBJ@D?@VnAXlC@x@?Z?D?NAnA?hD?~@?xD?tC@`B@h@@f@D`A?BDn@Dp@L~AN~ADd@FV@FFZJd@t@pDR|@Nl@F^FXF`@NhANrABR@PHjAPvB@PPxAF^F^Nr@h@rBhA~C|@~Bf@|AFRXvAHd@BVFd@B\\\\Dd@Bh@@^@n@?T?R?N?L?J?HAH?HAb@Cd@Gp@KdAOtAQnAaBlMYhDANA^?d@@|A@VD|@Dd@DZL|@Lx@Jl@b@hCNz@^vB@LDd@@B@PBZ@R@V@H?R@V@t@?P?X?h@C~@?@Cp@ANOlBALEh@AFGr@Gr@?@Iv@?@]~DM`AGb@UhAOn@Oh@]lAM^Y~@Md@Ur@g@bBUv@_@rAu@`C]jAOb@IXSr@_@lAOj@Yz@W|@Ut@Od@Oj@M`@Up@]dASj@m@rAaAxBO^{@rBUz@Sr@YhAUdAKt@QhAGb@Eh@I|@EbAAJCr@AnAAf@?zA@p@@FDhAHjA?@Ht@RvALp@Tr@FTXn@\\\\t@b@t@^d@LLTTZV^Vb@VHBd@PRFVDXF\\\\B~@BH?tBC`@?Z@@?bAPRBh@PFB^LTLLHLJTNx@l@BBz@v@NLHHDDNLHFp@h@JJTT`BzAFDNPVZLNPX`@t@FJP^Xp@Rp@Nj@BJNn@TrA?@j@rD\\\\vBVdBz@xFJr@l@fE?@NrAHjAHvA?HD~@Br@Ax@?`AMdG?ZEfACrAAz@?f@@n@@L@h@FdALpAL~@N~@J`@Nl@FTX|@Tp@Rh@v@~Ar@pAn@dAx@tA`@p@j@~@Xt@BFXt@Vx@`@dBZvA^tBX|A^rBXtA@FRpAbAzGZxCHnA\\\\pCnA|It@vFBPBNF`@|@hGx@dFPn@V`A`@hAd@fAl@`Ar@|@t@x@RR^Zz@f@~@^j@LPB`APj@FVD~AVhARt@LRD@@p@Xl@Zp@j@d@d@t@bAd@z@FNXt@XdAVfALp@PpARhBFn@@JDb@Hp@DXDVBN@HFXXlANd@Pb@@@Rl@NVFH\\\\n@R\\\\h@v@JRPTFHDFTZVZZZRPZVZRb@P^NPFJDr@RlA^z@TLDPDr@Rj@LPFND`Bd@j@Pt@TLDh@N`AVz@VrA`@b@LVLVJl@Zx@h@t@t@jBrBl@p@`@d@r@x@Z^f@b@h@\\\\n@Zn@ZPHf@Tv@d@n@j@VV\\\\d@j@|@b@bA\\\\bAVbANbAF\\\\Jt@JpARrBDXFl@ThAXjAPh@JT^~@b@z@LRRZVZXZVVLJNLRL@@VN\\\\R^Nx@X|Ah@dBl@DBp@TpAf@JFt@`@RLXPf@^NLRPJHLLb@b@RTJJj@r@l@z@j@bAb@z@Vl@Tj@^fA`@pANf@\\\\lA`AtDlCzJBL`BhG`@|AVbAHd@F^F^D`@D`@D`@B^B`@@^@`@@`@?p@Av@Av@ClB?`@?H@h@?n@@f@B`@@XBd@@B@PFh@NtADXNjA^pCPdBN`B?JDh@Br@?JBn@?^?X?FAfAC~@EbAAFEn@Eh@CVEZABEZETO|@o@zCOp@Qt@_@tAm@~B]lAI\\\\K^Kd@AFKl@If@Gj@AJEXAZAXCVAr@Cf@?p@@P@f@Dh@BTBJF`@Hl@Hd@Hd@Nz@Hf@PdAD^@FDZBVBX@R@R@V@Z@\\\\?F@T?RAP?d@AZATCRAZARETGh@Ij@GZKb@IZMf@M`@Qj@KTIRKVOXMTMVQTUXKLSTWVQN_A|@g@b@o@l@_@`@a@d@Y`@Yb@Wh@S`@M^K\\\\IXK`@I\\\\Kj@G^E^CZEj@Cj@Ad@A`@@t@?\\\\B\\\\@`@Dl@Ff@Fj@Hl@Jj@Hh@Hd@Hb@FVJd@H`@J`@H\\\\Jb@DJ^pAVz@Rj@Pf@DJL^BHRj@Nb@Pj@Rf@f@zAdAzCVt@|ArEFNNb@h@|Ap@hBl@tAN^j@pAVh@LXLXNVVf@V`@T^HJZd@fCnDV^@B\\\\d@h@p@d@h@VVf@f@~BbBXPdDjB@@`Aj@HFb@Zh@f@v@fAzBzCLNFJPVDDj@p@^n@DF\\\\p@Xp@\\\\hAT`ADNHf@Lv@H|@Dz@B|@@f@Ad@Ar@Aj@ALEn@UnDG~@MxCA\\\\G|AMnEO|E?BKvDGjBGfBKtCEbBGpBEfAAp@?hA@d@?h@@TD`AJlALhARrAFXP`ApAfG^nBNdAJ|@H|@D|@@~@?bACtAKbB_AzPO|CQzCAN?LUfD?DIvAIbACVQtAUvAu@|EYrBWpBQpBMrBIrBCpBArBBtBFrBJpBLvBL~@Fr@TdBJl@?BJn@Lp@Nn@Nh@Nj@Tx@Tt@HXj@jBl@dBl@bBhA`Db@tA^xA\\\\zAX`BF`@LbAPfBLbBJhB\\\\pHPjD\\\\|HTzEL`CJxBJpBFlBBpBFvIDnBJlBLlBThBVfBZbB^jBd@jBjAzEf@vCLv@Hz@Hx@NhBJ`BFnBDjBDdGBdCFnBJlBNhBLbAFd@VhB^dB@@Lp@Nl@FNFVLd@X|@JZN\\\\\\\\r@P`@p@pAt@nAr@jA|@fA|@~@z@x@`Ax@`@Xx@j@f@Z^R`@VnAd@z@VxAf@pBh@tAb@nA\\\\h@Nx@Rn@Rf@P`@TXPBBXPZRXP`@TTLVJTHJDND\\\\HVFF@h@Bl@?^A`@ENERGn@Ub@SFEd@Wx@y@l@u@DEz@gAv@_ATQ@CXSb@[FCXKb@QTGl@Kh@Ej@AH@^Bh@Hj@Ln@RbA\\\\|Ah@`Bl@VJpAd@`FbBn@Rp@Rb@Jt@Nx@LjBX|@PrBXnDh@pANX@x@FfBFtCJJ?h@Dt@Hl@Nl@Tj@Zf@`@h@d@^d@^h@PXP^N^Xt@ZdAZpAT`AXz@Jb@T`AVvANdAJz@JnA?f@JlBxAfNBA\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 47.5463596,\n                        \"lng\" : 7.592225\n                     },\n                     \"transit_details\" : \n                     {\n                        \"arrival_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 47.3619469,\n                              \"lng\" : 7.350569699999999\n                           },\n                           \"name\" : \"Delemonte\"\n                        },\n                        \"arrival_time\" : \n                        {\n                           \"text\" : \"4:24 AM\",\n                           \"time_zone\" : \"Europe/Zurich\",\n                           \"value\" : 1728699840\n                        },\n                        \"departure_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 47.5463596,\n                              \"lng\" : 7.592225\n                           },\n                           \"name\" : \"Basel SBB\"\n                        },\n                        \"departure_time\" : \n                        {\n                           \"text\" : \"3:45 AM\",\n                           \"time_zone\" : \"Europe/Zurich\",\n                           \"value\" : 1728697500\n                        },\n                        \"headsign\" : \"Delémont\",\n                        \"line\" : \n                        {\n                           \"agencies\" : \n                           [\n                              {\n                                 \"name\" : \"Schweizerische Bundesbahnen\",\n                                 \"phone\" : \"011 41 848 446 688\",\n                                 \"url\" : \"http://www.sbb.ch/en/timetable\"\n                              }\n                           ],\n                           \"color\" : \"#193969\",\n                           \"short_name\" : \"S3\",\n                           \"text_color\" : \"#ffffff\",\n                           \"vehicle\" : \n                           {\n                              \"icon\" : \"//maps.gstatic.com/mapfiles/transit/iw2/6/ch-zurich-train.png\",\n                              \"name\" : \"Commuter train\",\n                              \"type\" : \"COMMUTER_TRAIN\"\n                           }\n                        },\n                        \"num_stops\" : 9\n                     },\n                     \"travel_mode\" : \"TRANSIT\"\n                  },\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"0.1 km\",\n                        \"value\" : 122\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"2 mins\",\n                        \"value\" : 122\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 47.3609203,\n                        \"lng\" : 7.3510347\n                     },\n                     \"html_instructions\" : \"Walk to Delemonte\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"uja`Hwqzk@|DeD\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 47.3618683,\n                        \"lng\" : 7.3502005\n                     },\n                     \"steps\" : \n                     [\n                        {\n                           \"distance\" : \n                           {\n                              \"text\" : \"0.1 km\",\n                              \"value\" : 122\n                           },\n                           \"duration\" : \n                           {\n                              \"text\" : \"2 mins\",\n                              \"value\" : 122\n                           },\n                           \"end_location\" : \n                           {\n                              \"lat\" : 47.3609203,\n                              \"lng\" : 7.3510347\n                           },\n                           \"polyline\" : \n                           {\n                              \"points\" : \"uja`Hwqzk@|DeD\"\n                           },\n                           \"start_location\" : \n                           {\n                              \"lat\" : 47.3618683,\n                              \"lng\" : 7.3502005\n                           },\n                           \"travel_mode\" : \"WALKING\"\n                        }\n                     ],\n                     \"travel_mode\" : \"WALKING\"\n                  },\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"54.6 km\",\n                        \"value\" : 54595\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"1 hour 7 mins\",\n                        \"value\" : 4020\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 47.58587,\n                        \"lng\" : 6.89767\n                     },\n                     \"html_instructions\" : \"Train towards 18160\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"oka`H}pzk@LENzAXnC@HJ|@PhAP~@VtAb@dCRbA@FNn@Rx@Vt@Xt@\\\\p@vAbCj@p@HLHNJRZh@b@~@NXJR|EhInF`J|BpDbP`WvBfDnAjBX`@LRDDvA|Bh@x@Xb@fBpCdA`B|A~BDJvI~M|BpD`BhCnCjEzBzD`AjBf@`At@|AlBbEhBjEp@bBdApCv@xBt@vBTv@Rr@v@fCzDpNlGpTjBtGzMnd@d@dBHR@FpBfHd@fBR`ALl@^pBZrBVxBRtBNzBLvBF|B@vA@f@@fCBlFNjQFrGFpDBnABt@DtB@tAAfAA|@EhAMpBU`BOfA[bB_@zAu@fCQh@AB?@ADIV{@|C_@zBA@QtAIpAKxBGdC?X?TB~TB|R?fD?~A?j@BrTLrdADh^@lCB`DB|B?tC@fG@P?vD?D@`A?HBv@@TF`BB`@@XDn@RzD@XJbCXdGBXFnA?RDr@fA|UBXDx@j@lMDp@rB`c@n@vMDt@b@lJPlDNzCNzC^zH^xHBp@HhBBxA@`BCvAGxAMvACTK~@U~AWxAK|@G|@Gt@OdASbA}BdJc@`Bo@nCu@|Bs@rBM`@u@xCc@rAUp@kDrKaEbM_E|LmBbGaYpy@Qj@iCtHSj@uBjGSj@cB|EQj@wBhGQl@cB|ESj@aB|ESj@aB|ESj@aB|ESj@[v@g@r@aA`AgAf@QHODa@JI@i@J[D[?c@A_@ACA]Ca@IA?a@Kc@Ka@Kc@KC?oAWo@GI?c@CC?_@?]@{@Ds@JG?{@P}@Xk@XKDeAh@{@^kJjFkCtAcBz@kBhAyAv@q@^qBt@g@H_@DWDo@DeADc@Bm@BYBe@Bu@Hq@PC?eCz@YHk@ReBn@]Js@Xq@\\\\o@b@_@ZQPKJ[^[`@Y`@Yf@Wd@Uh@]z@_@pAGVGTSbA?FE\\\\[jDKv@E^CPGd@Ed@CRSnAGd@Id@Ix@Gz@Ez@CnAEhB?NApA@dADtAHrBDr@D~@BfA@`AA~@?@A`AIxAShAw@~F_@vCUdBMnAKv@G~@CbA?X?b@@|@D|@Fz@Fh@PvAJ`AJbAF|@B|@@NDjBBnB?p@]zBU~Ae@pCm@dCs@|AiB~Bu@l@eAx@ULwNjIKFMHeGrDcEdCgDpBYPeIzEeAl@uG~DmAr@c@Vw@`@q@^_DlBmNxI{PdKa@VsBnAsShMWPyEhCULmAr@oI`FqAx@iA~@{@~@{@lAe@t@g@dAeCtG_@fAoCpHs@nB_AhCo@`B]r@]p@a@l@ED]d@e@d@e@b@g@^i@Zi@Xk@Ry@Rw@Ju@Fw@Aw@Eu@Ku@SKEg@Q[OWMm@c@o@g@k@s@m@w@UY_@k@{@oAa@k@e@w@aAeBi@w@kBuCe@s@_@i@U[e@o@i@o@GIQQII[WUQAAa@UQK[MOEe@Me@Ko@Cm@A]@eB?s@Dk@Fk@Lk@Nk@Vm@\\\\OJ}@p@C@{@p@GDEDg@Zg@XSJYLo@Rq@Nq@Hq@By@?E?yAEw@Ay@@y@DSBc@Hy@Pm@RGBYL[Nu@b@OLa@Zq@n@a@h@KJk@x@i@~@e@`Ac@dA]fAYjAUbAOfAIn@CRIdAIrAEtAA~AMr_@W~q@?lB@nBFlBHlBPlBRlBVdB|ArJPjA@HLbAFnAFhA?zAAvAEfA[zHO`DEtAa@zIAb@ARAz@@tAFzBpAji@@j@@p@?t@At@Cx@Gx@Il@Kr@Mp@Qt@ABOd@AFSf@Wj@Yf@Y`@a@d@[V]TIFQJ]LQDUHUDO@k@Be@Ae@Ek@Kw@ScBc@SGw@SaL}Cg@KiD_AgAWOCq@Ko@Es@Bm@DSBKBUBWHWHYLa@REB}CjBA@mElCuCfBaDnBaB`Ay@j@EFq@l@y@x@i@r@_A`B]v@eAbCyCxHgDnIeApC_A`CELOb@Sj@GZEVIl@Kx@ABKhAG`B@jABbA?PJpANbARbAr@bDXfAX`BHj@Ff@Bf@@FBp@@d@A`@?^?p@Cr@I`AEb@A@In@Mr@A@Qp@Od@A@i@rAYh@c@p@KJ[^c@`@WVsDpDgGbGs@p@y@l@g@\\\\c@Tc@RG@]L]J]HaANu@N}Ch@gALe@Be@@g@AIAWAYEWEe@K[KYMoAi@_@Q_FyBmAe@}@Qw@KiAA{@DQBiATiGbBmATkAHs@@Q?kAEwESM?kAIeAAu@@u@Du@Jy@Rm@Pq@Vs@^}@l@iD|B}B~A}EhDcBhA{BbB_@VE?_UpOmAl@{@^}@^iA`@[HWHm@Nq@N_APeCZ_IdAwFt@q@Hi@Lk@Li@P_@NC@iFvCYPg@VYRWTYVIJMNUZMR_@n@[r@MZOb@St@GVUhAK|@IbAGz@ANI~CQ`GInAMlAWhB[`Be@`Bi@zAy@bBEJ{D`IsBfEeIdP{@xAq@fAwBlCqAtAgF|EiH|GCBiAdA}B~BqA|AgAvAs@|@oCpDqCpDkA|ASV_AjAw@bA[`@u@bA]`@u@bAWZg@r@QVY`@Yd@MRa@x@g@hAYt@GP[bAU`AmAhG]bBUdA[bAa@bA[j@MROVa@f@CD_@ZSNIFOLOH]RC@u@Ta@Lg@D]@Q@a@?a@?aBGaBIi@@]@a@B]D]FE@o@PODw@VKDc@Ly@VIBSHSFeA^cA\\\\cCz@q@No@Lq@DmAHiCNi@?e@?c@CyBQiAG}@?s@Bo@Fq@Ji@Li@Nk@Vg@R]LeAj@eAj@aClAeD`B_Af@i@XkAv@IFGF{BhBu@v@k@f@g@t@e@p@iAjAyBtBkC~BHXy}BrcAifAz|Bg@SCJOhAGXe@vCc@rCM|@k@jD]pBYbBOv@Ij@CNCNKp@k@hDKp@Kr@s@dEMp@Kn@e@dCc@nBMl@Mh@a@dBMt@cAtDAF{@dDGV{@fDMd@oArEwA|FUz@qAdFu@pCaBvGg@tBc@bBg@hBAH_@xAg@lB_@zAMd@Uz@WdAa@|AU`A]tAU|@[pA]nBUvAG\\\\MdAGf@OfAQxA?@WdBADUfBQrA_@vCKz@SxAAJYzBKx@CTSzA_@pCM`AMbAObAIp@OpASvA?@U`BM|@Eb@AFMv@Q~AIp@Ij@QtAQhAK~@SvAGh@S~AQdAIl@Gd@WhBUbBM~@MvAK~AABCr@ANCh@?JEv@AbB?`A?N@`@@`@DbADpA?j@?ZAbACl@?@GfAAXERC\\\\Kt@If@Oz@WbASp@]|@e@|@_@n@e@p@Y^]ZWR]V[PWLe@RC@_@LSBMDm@HQ@YBa@@[Bs@@y@Dg@B]Bw@@]@O@O@i@Bi@BcA@mACE?ICYAYAeAEu@CO?SAy@Aq@Ac@@o@@aAD[DG?E@G?i@Hm@Ha@H_@FeAXC?y@Vm@RUHkAf@C@GBGBc@P_@PQHOFq@XaA`@{@Z[H_@LqARq@FY@c@@G?u@C}@IkAS{@Ww@_@AA]SYQEC_@Ya@[w@{@Y[qBiCIMqAcByAqBKKqAaBaByAkAq@sCcAk@K_@GiAGi@CaABW@sANa@DgAZYHC@gAd@OFo@b@WPkAdAq@r@Y^o@|@ILq@lAqA~BaAdB}@|AMRk@hACDk@hAQZmA~BYd@g@x@KNOTaBjBg@j@mAnAeAhAiBlBa@`@kB~By@`Ac@j@o@v@sAbBaAnAQTwD|E}CzD_AlA[`@u@~@MP]d@qA~Ag@n@k@t@q@z@m@v@k@r@k@v@IL_@j@k@`AIN_@t@EJWl@Sd@Yx@[fAADYdAGVMl@WzAWdBK~@G|@Ev@Ev@A`@?T?p@?n@@n@Bv@Fr@Fp@Fl@Hl@Hl@Lj@Jh@DN`A~DH^d@lBd@rBdChKPt@Ln@Fj@@PBTBd@@h@?h@Ah@Ch@Ef@Ed@G`@I\\\\I`@CHGTKXO\\\\CHILMVQXUZKN[^W\\\\ABIHb@`A\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 47.36199999999999,\n                        \"lng\" : 7.35007\n                     },\n                     \"transit_details\" : \n                     {\n                        \"arrival_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 47.58587,\n                              \"lng\" : 6.89767\n                           },\n                           \"name\" : \"Belfort - Montbéliard\"\n                        },\n                        \"arrival_time\" : \n                        {\n                           \"text\" : \"5:56 AM\",\n                           \"time_zone\" : \"Europe/Paris\",\n                           \"value\" : 1728705360\n                        },\n                        \"departure_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 47.36199999999999,\n                              \"lng\" : 7.35007\n                           },\n                           \"name\" : \"Delemonte\"\n                        },\n                        \"departure_time\" : \n                        {\n                           \"text\" : \"4:49 AM\",\n                           \"time_zone\" : \"Europe/Zurich\",\n                           \"value\" : 1728701340\n                        },\n                        \"headsign\" : \"18160\",\n                        \"line\" : \n                        {\n                           \"agencies\" : \n                           [\n                              {\n                                 \"name\" : \"TER Bourgogne-Franche-Comté\",\n                                 \"phone\" : \"011 33 1 84 94 36 35\",\n                                 \"url\" : \"https://www.ter.sncf.com/bourgogne-franche-comte\"\n                              }\n                           ],\n                           \"color\" : \"#0745f2\",\n                           \"name\" : \"Belfort - Delle\",\n                           \"short_name\" : \"TER\",\n                           \"text_color\" : \"#ffffff\",\n                           \"vehicle\" : \n                           {\n                              \"icon\" : \"//maps.gstatic.com/mapfiles/transit/iw2/6/rail2.png\",\n                              \"name\" : \"Train\",\n                              \"type\" : \"HEAVY_RAIL\"\n                           }\n                        },\n                        \"num_stops\" : 16\n                     },\n                     \"travel_mode\" : \"TRANSIT\"\n                  },\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"443 km\",\n                        \"value\" : 443097\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"2 hours 37 mins\",\n                        \"value\" : 9420\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 48.84494,\n                        \"lng\" : 2.37348\n                     },\n                     \"html_instructions\" : \"Train towards 6700\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"{}laHkgbi@eCjAj@jD\\\\vB@D^|B|@xF`ArFDTx@pERvA^lCZnBLl@PhA^|B\\\\tB^zBZxB|@`Gt@~EVdBVhBj@lDt@pEx@fF|@lFbA`GTtAn@zDdAjG|@fF@Fv@rEHb@x@|ERjAJr@Ln@p@fEb@bCN|@ThA^xBf@nC`AfFDPz@jEXxA^fBx@zDlAxFj@lC^`BlAlF^~An@lCnAhFpAhFpAhFvAtFz@~Cb@zAf@fBj@lBf@bBb@|Af@dBf@fBl@pBn@|Bt@hCPn@`@tAr@dCr@fCh@hBFPp@|Bn@zBBDj@nBn@rB?@l@nBj@nBj@nBl@rBl@vBZfANf@j@tBl@vBh@pBh@pBd@jBd@jBHXXpAd@jBd@pBb@lBb@lB`@jBh@fCDPTjA@B`@vBTjAH`@P~@Nv@^lBZlB\\\\nB^zBF^TtABLXjBT|AFd@L~@NbAZ|BZzBXzBXxBX|BX|BFn@LhANnAFj@T|BTxBBNPlBPpBTbCP`CB`@L`BP`CB^J~ALvBJbBBTLzBJtBJvBBb@H~AJbCHbCHrB?LH`CHbCF`CF~BVzRBl@@n@DlCFtEBpA@VL~HD|CJhGDhBFtE@LJnHXhQ@t@HrE@t@^bX?bB@~C@hB@bE?J?h@?dC@X@t@F|C@t@HrE@t@JrE@t@F|C@t@@t@Br@DhB@t@Bt@@r@DhBDrB@t@DhBV|MF~CHvD@ZBhBPhLL~H@r@BbB?FJ|H?t@D~CHjG@r@N~IBtBB|@@j@?F@t@Br@?@Br@@r@H|E@r@@t@Br@@t@@t@BhBF~CBhBBhBBt@@r@@t@BhB@`A@r@@t@@t@@l@b@nZF|DLtJPfL@t@@p@HvEDhBHrE@t@JpFf@xb@\\\\xRD~CBvABzBDhG@z@@z@BlC@fABxAX`RR~L?TJ~H@hB@t@DtC?H?tA@jB?b@@dF@B?lBAhF?^@tE?D?~A@vEHbGD|BDnE@dAHlE@tEDhEBdB?B@t@@~C@r@BtD?P@xA@zCDvCFtGBnG@xC@lB?t@?N?pA?F@l@?F@v@@p@BhB@dABxALbG?TDfBFjBR|FLrERfGRfGPhGRfGRhGT|Hb@`NFbBl@zR`@|MdB`j@p@vNJjBZfGXdGDt@x@rPRlD@ZBZ^`HBb@FdAN|CDr@N|CBr@JjBBr@Bf@Bf@LlDL|CFhBBj@TpGL|CLhD\\\\zKLxDTvILvE^pJt@nOTlEVjFXbHPhEHdARzCp@zKXlETpEJ|An@|IFr@B^HfAFr@Fr@Fr@Fr@Fr@Fr@PrBVzCv@hKbAvLNhBjAdOPxB`AhN`@vFb@rHBb@`@bHb@`ITpHDpAFfBBt@@t@H|C?FD`BFbCVxN?RFjHBpE@`DBjLCrEG~HAfCInHCn@Al@Y|OCnAOhLAXMfGCpAWrJGhBItDCpACt@Ah@?DAl@IxEU`O?ZAbBErE?ZCnFAr@ArCGjI?t@AJEvKC`N?`DApKE`KCzH@xB?`BEfIGtEIfFGjCS|HAb@QpICfBAd@?b@EhEIpGG|FAvBGzGElBA^U|HE~AErBAr@E|AC`AAt@GxDGlCItCI|CAVO`FCxBCrAATCxDCdCA~B?rF?tF?TDhG?RDdFBnBHrG?RRxJVtJNjFPrEBh@TnF\\\\fGV|DXnDd@pFBXj@zGFn@`@rE`@fEZhDz@hIf@dE\\\\nCVpBZ|B`@vCd@tCTxAXlBHh@j@zCBPn@rDZbBf@pCb@|BTdAz@hErAnGRz@v@nDDNlAjFvAtFn@dC`@zAb@|An@|Bz@vC`@rAtAtEHTpAdE^fAhAhDbBhFFPdA`DTp@v@~Bx@fCd@xAdAzCfBvF@@rAhEr@tBb@pA^fAjBvFhAlDd@tA|AxEpAzDNd@zAtEl@jBz@hCnBdG@Bd@xAx@fCBDNd@x@fCVt@`@nA~@lCbBjFlBzFrBhGFRxB|GVt@tAhEHTRl@x@fCtAjExAvE\\\\lAfAnDjA|Dd@xAzAvFlAdEdAzD|@bDl@dCp@lCXlAvAnGJ`@r@lDt@fDv@zDj@tCLr@h@nCHb@jA|F?Bx@jEt@zDLp@Nt@j@tCfAnFj@vC`@nBHZx@~D?@lAnFR`Af@~BT|@fA`F@DdAjEx@dDPp@`@~A`@zAp@bCt@rCNn@Pl@~AbGVbAPl@`@|AFTHVNl@b@|APl@DPJZPl@Nn@^nAr@dCXz@dAjDVv@p@xBTp@BHLb@hAdDJ^jAhDnApDj@|An@hBLXf@vAf@pArBtFHRRj@v@tBpAdDp@hBTh@b@jAxAvDTl@`BnEn@zAr@hBlBfFnBhFjB~Ev@rBl@~Ax@vB|@`CDJN^f@vARj@Rh@dB|ENd@`BdFpAhEn@vBl@rBnB|GTz@t@vCJb@lAdFr@tCNn@Jd@rAlFrAtFNn@BLrAjGjAlGn@bEXpBt@zE@BJp@j@lDf@pCf@`Dt@xFXxB\\\\lCh@rE?@PfBHp@Fr@J`ADd@Fr@Hr@NdBFr@BNDb@NfBPtB^zEd@`Id@xHZbFFfA^lHHpBJ|CLtDHnDFhB@b@@NBhB@t@DjC@RBhB?JJxFDzGApJ?|A?jB?hB?@AnA?z@AbBE~D?hAChB?t@Cr@GbCA`@?PANChBEjBCpAU|ICj@GzBO~DWdGIbCWhGIdCE|@O|D]~H[rHEdACr@A\\\\G`CIdCGtCA`@EhBE`C?^AX?X?d@CxBCvC?F?hBAj@?|@?t@A~A@hCDjEBzABhBDlC@|B?H@zCAvB@hCDpDBxA@`A@f@DtA@j@?F@d@@ZDlADhBBh@HzBVpEPtCJfBBXHnAJfBFr@HxA@NLfBLfBB\\\\f@pG@FLfBLtAFr@Df@@PRvBPfB?@Hn@N|Al@fEr@dFZdCv@|F?@V`BVbBJp@NbATrAV`BBNH`@Lz@RhAThAdB|IH^TjAb@tBLn@j@tCz@`E\\\\zAFTFXv@~CjAlEv@tCZfAdAnDjA|DtB|GBDvApEbArCRh@Tl@Rh@f@vAJVBHRj@h@tABFj@rAJVjAxC~ApD`BxDXl@l@rAVf@P^DFTh@n@nAFPLTHPr@vAVf@Vf@Vf@@@Td@^r@~@`BtBvDvAxBDFP\\\\dBtCXd@LThC`EhBlCx@jAzCfElDxEX^Z`@X`@Z`@Zb@Z`@b@j@l@x@Z`@Z`@zAtBxC~DZ`@BDpBlC`BxBpBlCXb@Z`@Xb@v@bABFJLNRj@v@JPZ`@Xb@Z`@j@x@pB|C|A`ClDvFpCvEVd@z@vAxFrKrC`GfA|BpBfEfAdCpE`KjApCpC|GvA~DtB`GnAzDd@xAv@fCRj@?@b@xAPl@Pl@Pl@Pl@Pl@b@zA`BtFhB|G|AlGv@jD@Jj@nCFXn@jDVzA@DLp@Ln@XbBDRDRZbBJp@Lp@Ln@\\\\jBXvAj@tCn@nDPbAf@rCXbBb@rCJp@Lp@Jp@Hj@@DJp@PbAXpA^zA^xARx@XfAjAjGLn@`AhFpB|LN~@XbBLn@VbBF\\\\D\\\\b@fC@HX`BF^r@tDVnANn@Z`BJd@@DJh@^fB\\\\`BRdAvB`LZbB~A|IhD|Rt@hEX|AHb@?@t@dE|DnTpAjHzFt[Jl@Jp@|Fr]d@nC`BdLzA`KLz@f@xD~AtJlAtIzAhKpBtNRzA~@dGPdAzAdKRzAn@xEpAlIF\\\\r@rEbA~FnAtHf@fD^vBpCbPlApGR~@P|@j@~Bn@nC@@fBhIz@pErCxMTdAnE`StBpIbDxMhBdH`@zAPn@Nl@HXTdAH^DNr@jCPl@Nl@b@|A|CjL`@pAt@fCxC~Jb@zARl@Pl@t@fCDLJ^d@zAPl@Pl@FPJXfCxHxElNxF`OhDvIDLDJ~BbGjAzCxD|I~F`N~D~I`DrG@BpC|EzChFJR~BzDRZDHJRHJTX|B`DfJpMl@z@bGxIFF`@h@TXx@fAVZ\\\\^LPh@p@RVFFx@|@pIbJfCzB^ZrF~E\\\\Z^Z\\\\Z\\\\Zt@p@FDPNJJzArA^Z\\\\X|@v@lGrFjQbL`An@^Td@Zx@l@^XVRd@\\\\BBZR`@T~@l@~H`FjDdCvBxAjCjB|ZnSzAbA~AdANJLH`ElC^VfMnIh@`@pBpA~@l@BBzBxA`An@n@`@@@|@l@~B~AtCnBjCdBtA~@`BhA~@j@`GzDh@\\\\vFtDtBtA|DxCfAz@rB~A`FdE|ApAr@l@r@l@d@^b@`@jD`DbAfAfBdBzCbDDFjBrBb@h@hApA`IvJdDnEtAhBdBhCdCrDJNpCfEbA|AtEhHrAvBXb@rH|LXd@lI`NhAjBV^T^Xb@Xd@Xb@`CvDxAbCJLHL`CvD`CvDXb@rApBnBpCdB|BpAdBp@~@vBnCVZ~AnBxErFXZb@h@bChC`BdBXZzFvFjD`DtAlADD|FbFVRVRhGbFjA~@rDtCdBrAzC`C^ZzEtD|@r@|@r@^XfBvAn@d@~@n@j@^fBtAfA|@|BdB|C`C`BpAv@v@lBxAjCrB@@`GjFjAfAz@x@fCfCp@x@`A~@t@v@vAzAVXxEpF`DzDhBbCr@~@vCdEvChE~EhI~IbOrDjHrArCVf@Rb@~BbFlArCl@tARf@vAhDJX\\\\z@b@jAd@nABF`@fAf@vATh@Rj@\\\\~@nBrF@BNf@lArDjArDd@zApAlEd@dB~@jDfAfERx@bAdEVjAf@zBhAdFzBnKn@`Dj@pCZ`BJ^H`@h@pCPz@F`@pDbQdAdFXpAbBrIdAnFNr@j@pCdAhFv@tDHb@Nt@pAlG|AhHtAvGLn@xBnKj@rC@DLp@\\\\~AZ~AZ~AXtA~BpLBNjA~FdAbFH\\\\|@tEFVFXr@lDd@fC|@bFrA~Hz@rFBNv@vFrArJjAnJTjBZvC\\\\xCHp@Hr@R~AHv@ZxC\\\\xCRpBv@dHb@bEnA`LbAbJRbBZzChDv[p@~F|BvSjAbKNdAn@vE~AvLFj@bBtKTxA@H`@tCNp@DRF\\\\b@dCf@rCtErWrCzObAtFl@dDjArG|AvIpAbHvCbQL`Ab@vC`@tCDZHl@b@tCXbB@JfAlIx@xH\\\\|Ct@nHh@jFv@|JJpAf@|GFx@Fx@TpC?FZdGRvD|@hR\\\\`JFhBL|CBd@@b@XfGRtE^fJDz@DnAp@`QZzH?H^dJB`@XvIXvJJbHB|@D|CBtA@hA?r@?RDjC@r@@rA?jA@nB?l@@t@@|C@t@BrE@hB@rCDfILtTVnTDdBn@bULfCt@tOl@`J^pFBPh@vHr@rITzBf@xEPxAp@|FBPf@vDFf@VnBF`@XrBTdBr@dF|AnJ\\\\zBrAfH|@xEl@~Cv@rDDRdBzHXrARx@n@lCPn@Nn@f@tBXdA`@zAb@`BjBlGHZtAvEjAfDt@xB@DRl@nD|Kn@jBbCpGp@hBnEpKTh@v@lBHPTf@nBnEtHbQvB`FfAdCrA|CxCdHlBnEh@tAvFvNhFfPd@xAzB`I@DbArDr@hCDNhA|EbAnE~BhKz@vEh@rCx@rEH^X`B|@xFJj@ZpB`@jCj@~Db@~CNjAb@pDJr@l@|FDVj@lFN~Aj@fG^jFfAnQBj@d@hIVrHNzEJ|FDhBFzD?VJfGDnC?NFdJHzTB|H?t@@|C@hBB|HDfLBrE@~CBlB?D?r@@t@HlDH|CRvILvD^dLJrBh@bLJfBBh@F|@LfBLfBLfBLfBPnBPdBjAzLlAtJ|@nHlAhIlAnIzCxSvB`N|@xFb@rCp@fEb@tCX`BVbBNdAVbB`BnKHf@Lz@VbBV`BVbB|@xFLx@Jp@Jn@Jp@tA|IlB~LJp@Jp@b@tCPlADXJp@b@nCJp@Jn@XbBnB~LJp@Jp@X`BJp@VbBTtA?DJj@Jp@^bCd@tCbBlKjDvTlAxHlAxH`DbS~@vF|AfJPbARpAX`BX`Bf@tCXnB`@tCnA~IVbBHp@hAhHb@rCp@fEXbBJp@V`BVbBd@tCVbB|@vFP|ATdBn@jFLbAzAzMTfCjB|SJnAN~Bf@vHLfBLhBHfBVpGBf@F~CDfB@t@Br@DhBBfB@t@BhBNdLD|CB|E?r@?hB@dB?x@BfLMnQc@tOG~A_@nJOpDMnD{Bp`@gA`PYnEUzCSrCC\\\\ATGr@MfBKrAMfBa@bGsAnRi@vHMfBMfBKfCu@`QIjBIzCAr@MrEAr@I|CAt@IlDAb@Ct@G|CAr@EhBEhBIrECvACxBCrEAhB?J@|A@hB@hBBhB@vA?PF|CH~CJpE@t@B~@Bt@Br@DzAR~EXzHN|D@Rd@bLP`EVjDp@rJVpCxAvPNbBbB~Nz@pHZvCRdB\\\\`DHr@Fl@?BHr@Hp@ZxCHp@VbBbAlHnA~I`@tC^xC~@nHNhADXRdBTbBHr@Jp@Hp@Hr@Jp@Hp@J|@Hr@Hp@Hr@jAjKJ|@rAtKhA`J|@nHf@nDd@nDl@jF~AzMd@tDlCpSRbBJr@Hp@Jp@NjAFd@BJHp@Jp@LhAHp@Hp@Hr@Hp@Hr@p@|Ft@nGNrAxB`RRdBRdBZ|BHp@Fd@@JJp@Hr@TbBPvA^tC`BfMTbBHp@p@dFHp@Hp@Jr@Fb@@LHp@Jr@Hp@^vCTbBNpAJp@pBlPfAbJh@hE^vCHr@`@jDBLDb@Jr@Hp@Hp@Hr@Hr@PdBrDh]PdBHr@RdBB\\\\Db@@LHr@JvALfBj@xHn@hJDr@Fr@Dr@Dr@Bh@Dr@?DDl@Br@FxADr@Br@Dr@b@lKLhFNzHDhBDhB?|@?H@t@@nB?t@@fA?`@CfGAfBEtEAhB?r@AxAC~C?r@?`@?RAr@?t@Ar@?t@AfBCbACr@Ct@a@pOq@jM?Fa@lJW`G[hGEt@SnEKhBEr@Cr@Er@E|@Cp@?@Gr@Er@GbACr@IhBC^s@lSy@|PIfBGlAMbCWnEWpEKfBSfDG|AWdG]xHGtAQdFSfGGfBEfCE~CKfGG|CChBG|C?XCdC?j@ArE?`@LzGJrHB|@LzHBhB@r@LrEFhBTdITfJDhBDfBDhBDhBB|B@r@?RHtAHfBp@rI\\\\nEFr@Fr@Dr@Fr@RjBFr@RdBFp@BVLnAl@~F`@fDhB`PBL`CjU`B~OBRlAbO^nEt@hJB^zAdLtAfK@Jb@`EbAbJn@~Fx@pHlBlPRfBl@zFj@fFjAvK\\\\xCZvCHr@l@jFZlC@Hn@~Fn@|Fp@zGTtBxAfIz@xEzCvPlAbHVfBb@tCTbBnB|MHjAr@zL\\\\lL?dA@nD?lHKxGMpJErC_@bG_AhOMtBe@nHu@jLc@pGIfAK|AcApOgAdPAPAP_@dGGr@SzCEr@SzCEr@KfBy@dMm@rJg@~HUpDo@xJOxBw@|La@xGG|@UjDYtDa@~DIz@U`Bk@xD]vBMt@g@lCc@xB[vAi@vBi@nB]lAi@hBM\\\\mArDUn@_@dAYr@{@vBeCnFeBtDk@nA{CtGq@vAuAtCy@hBiA~BKV_@v@MXMXq@xA[l@Wf@MXu@~Ao@nAwCjGyB|Ew@`B_CbFm@pA}B~E_BjDGJ_D|GmAjCcAtB[n@g@hA_CbFuDhI[n@{AjDwHhPm@tAgCvFcA|B{A~Ce@hAaArBa@~@cAzBqChG}AdD[n@iCtFg@fAcAzBuBlEk@lAsBnEsA~CwA~CqAdCQ`@m@nAOZGLm@nAYn@S`@Uh@k@pAEHoA`C}@|ACDS`@cAzBWd@qCdGoAtCgAhC_C~EmCzFUh@m@pAqBjES`@oApCKZg@vAuDfIaAtBuBnE}@lBoC`GCDqDvIiA~CA@}AdFsAxEy@fDu@nDUhAcBvIsB~JkDlQeAfFo@fD]dBSbAq@bDgAxFg@`CsEtUq@hD}DbSg@dCuC~NuK`j@sBjK_CtLcBtIi@pCw@`EENI^k@pCCHe@fCuAbHIb@aCtLCJoBdKMn@{CvO}BvLiDhQ[|Aq@tDiA|F}@rEa@vBKj@ABi@rCi@pCWrAQ|@Mn@Q|@WrAMp@ER]dBGZ_@nBi@nCiAdGaAbFmAjGo@hD_@nB}@rEEXi@pCGXm@hDUlASdAKp@CJWtAOt@g@lCy@vEwAbICJeAhFyAbIu@~Dg@lCo@jDCNUpA[bBSdAGX]lBo@jDCJ[`B]jBWvAOx@Kh@Q~@}@dFm@`DQdAGTSbAG\\\\CPg@rCg@lCUxASfAI^CJWzAYzAKd@u@|DiAdGw@lEq@pDOv@{@zEs@vDo@lDy@jEMr@m@~Cc@dCOt@g@lCY`BQ~@g@pCc@xB_@xBY|AWzA_@`CG^Ij@Il@E`@E\\\\G^K~@MpAKjAMhAGr@ANGp@Gp@SxBQvBK`AEf@I~@IbAMxAMpAQtBObBI|@C\\\\KfAGh@WvCY|CM`BANU`CQvBKhAEb@CVGr@KdAI~@I~@KdAKlAI`AI|@I~@MvAQnBMvASvBQpB?DMlAIbAQrBS|BS|BStBOfBQjBQtBQpBMrAQnBIx@Gt@?BGn@Eh@I~@Ir@CZKjAMrAARO`BY~CABOjBEh@E`@MxAQnBMtA_@`Eo@tH]nD_@lEa@lEGr@_@lEi@`Gq@tH_@lEa@nEm@~GALADSzB[dDEd@Eh@MlBCLAH?JO~AKhAGd@]zDYbDWjCUpCY|BW`BUrAQ~@Ib@c@nBOj@k@rB_@`Ac@pACBuAzCm@jAmAjBu@dAmAvAML]^g@b@g@^{@l@u@b@QJeAf@OJYLwBdAsAn@_@PcAd@c@T_@PaAd@a@Pa@R[NgAh@]NC@aAd@k@XiBz@cBx@{Ar@aAb@m@Vm@R]JGBWHYF[F]Fo@Ho@Fk@Dq@BI?w@?k@Ca@AWAg@CEAa@C_@EC?WESCaAMaAOKCk@KYEOCKAKAOEGAEAWE[GEAy@Mm@IYEc@G[CEAc@GgAKUEOAIAq@IWCKAc@Ck@E[A_AEG?c@?c@?YAm@BU?M?Y@q@D]Bc@D[BG@gAJC?a@Dc@DWDqALK@aCTe@DUDA?_@DC?YBgAJsBR_@Fo@FWDe@F]FKBGBODQDc@NWJ}@Vk@Zm@^e@ZEBaA|@a@d@a@l@EFOTIPYj@Wp@GLWx@[hAS~@O`ASdBGt@K`AMjBK~BGfCCxA?jCFlCJvAJbBLtAJrADVJ~AFrADp@@R@X@X@j@@r@?hB?VEtDCp@?bB@p@A\\\\?RE~@?@APIp@?FIl@Gp@CNE`@ETS~AM~@M|@Q`ASdA?@YrAQn@e@hBIT]fA[|@CJeAjCEHOXmA~Bi@bA]p@IPw@tAk@dACFS^QZgArBKRc@z@S\\\\[n@MV_@x@qApCc@|@MVa@|@_@x@O\\\\IPCFIPYr@CFw@vB?@Up@Wx@_@tAOh@?B[pAQx@YbBCFUxAUbB?@CRCTGd@SnBKfAW`CKbAYpC?@OvAW|BSxAIp@Kn@?@G^CPABUpAA?Mn@GZERa@|AQl@Ux@a@lA]fA{@tBy@hB[j@{DbHgCpE_AbBqA|BINe@z@KPc@|@c@dAGLUj@]x@KZITIRQj@ITQj@m@nBK`@y@xCq@lCe@lBe@pBWjAe@lBK`@g@pBAFu@hCk@dBkA`DoC|GELIZMZ]x@aBfE}@dCQf@?@Sl@Oj@Ql@GPGXa@~AAFCJCJAFKh@Sz@Ib@Kb@CNSjAEVOlAOlAShBGp@CTATCd@IlAAVA`@EtAAl@Aj@A\\\\?N?L?`BDtEJjI@l@JlH?N@P?PJnF@lAFjEHhDDbA@b@FhA@P?D@H?\\\\BNB\\\\@JB\\\\BND`@@NFb@@NHh@DZF`@@HF`@PdALr@R`ARz@TdARr@Ld@HXXx@Tr@HTXx@`@dAJTRf@b@`A`@|@d@fAZr@BDZv@\\\\|@Xv@Nb@DJHVDLL`@HTJ`@HTHVBHPp@Pr@H\\\\H`@\\\\pAVhAf@|Bb@pBBRNt@@FZtA?@HXNj@Nj@@FFPTt@Rj@Rj@Rj@@Bf@pATj@Rb@d@hAZn@BDl@nABBl@hA\\\\l@pAjBjA`BfAxArAhBlAdBHJb@n@T^X`@T`@Zj@P^@?P^Vh@Tf@b@bA^~@Vj@\\\\x@b@fAb@dAr@~ATf@BBl@pAT^PX`@n@BDj@~@v@fAr@|@t@x@bAdA\\\\^l@n@JJz@x@Z`@TRt@|@BB\\\\\\\\f@f@RVXXTTr@z@JNr@|@BDx@fAx@rA|@~Al@fAl@rAd@jAp@lBPj@HXTv@b@|AV`A`@rBTnATtABPLz@Hv@LdADZ@XB`@Dt@@LDv@Bp@Dz@@j@@H@^?P?b@?P?r@?t@?hB?r@?h@?R?R@t@?r@?H@j@D|C@t@@l@?D@t@Bz@Bj@@f@NlDDlADnAHhBTdG`@vKDz@?N@L^rIDbALjDT~GNpE@DFbBBr@DlABnA@P@PPrEDz@?FBb@?@FfBLtCLdDFtANrDBd@@b@@V?BBd@@z@@F?P@`@Bt@Br@@r@FhB?X@ZBr@@d@@X@X@T@L@XBX@PB`@Bd@Dl@FhA@JBf@@PB`@Br@JhB@RTpF?HDxA@D@f@@^BnA?D?t@@H?t@Ah@?nA?P?FAb@Cv@IxBCp@?BEt@OtBE^AJQhBUbBKj@Ih@ABId@EXeArEAFc@|AIVm@fBUp@ELWn@KTe@dAGJOZQ^Yj@IPCBMVCDABS\\\\MRQVs@hAOTA@SVkAzAA@k@l@SRQRCB_@Zi@f@s@h@_Ap@_Al@}Ax@_Ab@qAn@sAt@A@sAn@gAp@wBbAgB~@aAj@yA|@sAdAa@`@]ZsAvAaAjAU\\\\s@fAs@bA[f@Wd@CBmA~Bo@zAIRo@hB_@dAIVYbAELCLMb@Ol@Mn@YrACJUlAERW|AWjBGp@K`AEd@Ir@KbBMzBCdAE`B?hBA|ABzBDnBLbCJzADt@B\\\\BRHr@@LNlA@HHp@BNJp@^dC?@\\\\~Ab@rB`@vArCtItAbE^fAXx@r@rB?B|@hC~@tCJVL`@Vz@Nh@J\\\\XbA^nADNJ^@DDLd@nBZpAVjAH`@b@zB`@tBR`ATjAPfAHj@Lv@PfAXbBV~AF\\\\PbAF\\\\Jl@F^@JPv@Hh@RjA^pBPt@Lh@@D`@|AJ`@l@rBRp@f@tAL\\\\\\\\`Af@xAn@dBf@tA~@hCFNRh@`@jAZx@Z~@@F\\\\fAXhAVz@XvAb@zBZjB@DLhAL|@LnAD`@H|@J`BL`C@TBtADlB?xB?f@ExBGbCGjAE|@QtBW|BShBEXi@bEi@nDc@~Ca@pCEV]tB[xBGb@c@zCYnBq@|EW`Be@dDCRg@jDWlBOdA]bCId@[|BIb@U|AWhBEV]zBY|AYtAEPQ`A[tAKh@EL_@jAELK^e@xA?Bq@rB_@nAc@vA_@rAa@jAADSv@g@fBa@tA]rAm@pBk@zA[v@k@lAADo@pAc@v@wA|Bu@jA[b@o@|@_AtAaB`CCD}@nAs@`AMRg@p@g@r@sArBIHoAjB?@}@nACDeAzAyAxBA@mAfBCBsAlBw@jAy@jAKPg@r@wArBuArBaDrE}@rAeCpDyBdDu@fAYb@EFi@p@_HdJY`@eDlEmBhCiChDMRwBzCeCpDcCpDeCpDeCnDeCpDeCpD_DrEeCpD_DrE{DxF{DvFuEzGuEzGuEzGoF|HqF~HeHdKYb@ka@pl@kL~Qc@r@mAjB?@gClDSR]f@q@|@e@n@cC~CoAdBg@p@y@jAgAzAILQV}@pAu@fAoAjB_C|DeBdC{B`DMR}BfD{@nAmAdBwCdEwB|CKNmC|DcC|Ck@n@c@d@e@d@sApAcBnAo@d@a@Z}Az@qAn@mBv@MDeAZWHaB`@s@Lm@LcCV_DZwCTqALyANa@DkBNcAHwBTk@DgAJoCVeAJyFf@{Ff@w@HgALq@F_BLqAJWBo@HeALs@Jw@Lq@Lw@N_APkAXcAXe@Na@N[Jq@RgAd@o@Tg@Tc@R{Av@kDlBOHeAp@{AhAkA~@YTc@^[Vk@f@SPUVGDs@t@o@p@QPuAxA]b@m@t@wBvCi@v@k@|@QVYd@iAhBo@dAw@rA]l@kAxBQXwClFi@|@qCdF_@p@CFe@z@]n@iAtBA@wB~DuAhCu@hBiBtE]z@_@~@]fAi@zAoBxFc@lAKTeAvCa@fAcBpDABw@fBwAtCi@`AQXmAjBGJ]l@aAtAs@bAu@|@s@|@e@n@GFg@h@s@v@u@v@q@n@gAfAs@p@EDe@\\\\q@h@kAx@c@^UNa@Xi@\\\\o@b@eBdAA@iAn@y@^OHgAf@SH}@^}An@iAd@A?cBp@IB_Ad@s@Ze@TCBo@Zo@^C@i@XmAx@q@h@URURqApAu@z@y@bAyB`DkCjF{BjGg@tAeB`FsB~FADy@bCaArCyAnEk@bBy@dCQd@[|@k@fBi@|Ak@`B{@hC[`A]dAcAnDOn@Qt@sAfFy@jDu@|CU|@gCxJk@jBg@xAk@dBUf@O^e@jAm@~Ac@bAs@~A]t@Ud@Wj@Ud@A@Wd@U^[j@MTKPWd@EFa@r@KNY`@KNMRY^[d@UXq@bAc@n@cApAwAfByA`BIHqAvAkDpDcAfAYXyD|DsBpBgAfA{A`BgAhAON}A`B{A`B}DvDqCdCk@f@_CdBq@b@C@}BvA_Aj@a@RcBx@gAh@_A^sCjA}Bv@yAh@uCdAoC`A}@\\\\iDnAMDgBd@eEbA{EjA{@TaDx@uBh@cEbAo@P}JhCgAZ_Bb@GBiCx@gBn@sAd@YLc@R{C`BcB~@k@^sA~@cBjA_ElDiBjBqApA_B`BsB`CuA~AuA`BEDUXGHCDiApA[^sA|AA@[`@QPc@l@]b@[^w@bAMNcClCwAxAUT{@~@aE|Du@n@sBdBcDhCc@\\\\yCtBYRQLeBfAoAx@cIjFgBjAyA~@oBnAgBjAiEtC}BxAm@b@wBzAuBvAg@^w@j@cBvAKHoAhAu@r@}@bAsB|BkA`BcA`BOTu@pAcAvBu@dB{@xBKZ_@jAELSl@Ql@GPIZENMb@ADm@~Be@pBIb@s@fDg@bCABe@hCi@zCMr@k@|CWvAe@jCe@fC{@rEi@rCm@hDG^i@|CkApGq@zDETQx@WtAq@tCMj@c@jBs@rCk@xBGTw@nC{@nC{@~B_AhCy@rBkAtCqAtCw@zAGLkA|ByAlCmArBwAzBqAtBiA`BqApB{A`CqDvF?@uBdD_AvAORo@dA_CnD}AdC[d@g@t@gAhBQVsArBS\\\\s@hAuAtBm@~@mBtCmA`B_BxB_BvBc@j@GLo@x@mA`Bm@x@]d@k@v@}ArB_AlA}A`CgAdBgAbBOTs@nAgAjBs@jA[h@mAxBoApBoAvBoAvBmAvB_DfFABuEvH{C`FwCfFwA|BiAjBeB|Cw@tA}CpFoBhDuCdFYd@sCfFcEpH}GzLiCtEk@`A{BhEsBfFgAfDWx@m@fCk@lCk@bDG\\\\SbB[jCUjCQfC?DKnCInCClCAfDEjD?n@AtD?xECjEA|GE~FAf@CvBErC?HCl@?FAZYrDMhBEf@K|@QxAa@dDgAdGq@lDqArF_BxGgDxLsApE_@rAkBdG{@~C_AfDk@jBWt@{@rCq@pBSp@qApDqAlD[|@sArD[z@eArCiA~CKXkAbDkA~COb@kA~CiCjHM\\\\aCbG}A`Eu@fB]v@a@fAu@jBk@tAs@fBg@jACHk@tA]x@iA|CGNu@pBKX_@hA_@pAOn@[lAe@xBc@`C_@fC[`CYrCGt@OdBMxBGxAE~@IrC?XCtBAb@?`@?|@?jCJ~DFbCPhCHjAFx@Fr@R|AZhC`@bC`AdFLp@j@~Bb@~Ah@fBPh@h@~Al@hBVt@n@bBb@pAx@dCBFd@pA`@rA\\\\jAb@zAZlA@B`@|AXjAn@rBj@tBRv@Tz@^pAZpAj@rCj@vD^hDNrBNnBLvDFvDDfD?`C?pD?n@CzACdAA~AAJGnBIzACRCXEZARAXARAdAArA?f@Ab@C~ACnBATE~AA`BCfBC~@GbBCnB?HCbACfAGvCGdBGpBOtDQbBQ~AEb@CRU~AKr@Kp@Mp@QfAIXWjAe@zAU`AAB[~@i@|Ae@fAa@`Ao@vAGNy@fBk@hASb@c@l@aB`CgCrCQP_@\\\\{ArAaAz@y@p@uHhG{BfB_@ZyEzDuDzCaCnBwFvEq@h@sI~GgFhEwBdBwDvDuCtCSR}C~CyB|BsFrEgBzAuIzFqA|@MJ_BhAi@^uBvAeAp@qBdBCBgCpCkDhEEFiE`F]`@aDvCq@j@}IjIeDxCoDfD_DjCkDtCmBtA}FfEWReFtDoHnFmAz@}DtCiDdCsA`AaEtC{AfAsIhGiDdCkIfGmEbDwFbEcDfC{BfBuB`BgBpAkBvAgEzD}CrCuDtDq@p@oDjDyA~A]`@s@v@aAjAiBzBkCrDy@jA_CdDiCrDw@dAaDpEsC`EoDbFu@dAmBjCYb@gAxAILoAhBcB`CqBrC{EzGu@fAkCxDkArBiDxGaAtBe@jA}AxDuA~DsAjE_@pAm@fCQp@a@fBOj@Kb@I`@]vBABoC~OyFp\\\\mAjHgBjK{AzIKp@Mp@Kp@Kh@Kh@Mp@Kp@Mn@Kp@_AjFi@`De@pCo@fEe@|CGh@ADK`AM~@OxAO`BQnB]lEQvBShCIjAKxAC^C`@Cr@Ev@CnAC`B?r@?n@@nCFlCFlAD`A@PDt@Fz@BVFn@JfAF`@D`@Jt@BZD^F^ZlCRfBn@vFr@fGXnC@Hf@lEXdC`@lDDXNhALz@Fh@NpABRL~@J~@H`ADh@JrAHxB@L@|@@pAB|A?fA?nACxA?FAfCAVEn@Ep@C`@I|@En@UvB}AdMoAxJs@pFGh@mAbJE\\\\e@`DWbB]vBg@fCYlAg@pBQr@Qr@_@vAUr@ELGPKXw@tBe@jAKTu@dBeAxBaAhBU`@]l@c@p@Yf@MPU\\\\u@bAA?m@t@}BlCqBbCqB~BuAbBsB`CoA|Ay@dAKLOTy@jA{@xAkFhJsA`CcA~A]j@q@`Aq@v@_AfAEDy@z@g@d@OLaDpCu@l@wCfC]ZcA~@y@z@Y\\\\a@d@UZGHW^UZYd@[d@]h@U^MRc@x@Uf@w@bBs@lBg@zAc@vASz@m@~B]dBGVEXWxAWjBShBCTC\\\\Ed@ALGr@CPG`AAHA`@ANCb@Af@ALC~@?h@At@AvA?f@?rA@fI?xC@nGBnED`K?lI?ZB~F?rB@xB?fE?rAAvCEtCEnAE`AANKvAG~@AHCNALK~@Ef@MlAEVIl@M~@Oz@UlAOt@On@y@tC[bAq@rB]bAADa@fAq@jBSd@O`@m@vA[p@Ub@QZa@t@a@n@Wb@OTMRUZQVOTILq@|@EFY`@i@t@e@t@W\\\\]h@ABm@|@i@z@m@`Ae@x@c@z@e@~@k@pA[t@MZa@nAIXaA~Cw@bDu@rDo@rE_@rCKhAYpCw@hIMzAKxAI~@QjBSjCa@fEWhCYlCABQ`BE`@QbAGh@E\\\\Kr@ALIb@Kp@Kl@Or@[bBMn@cA|EWbA{ArFcAhDeAzCg@vAmAtCqAzCEJEHm@pA[p@Q^qBnEKRmApCmApCO^ELGLiBxEKVmA`DwApEA@o@xBcAvDu@bDMl@WpACLCJI\\\\k@bDGZs@fE[vBKr@Ip@OlAi@hFWxCMfBEd@a@hIM`DK|CAf@KvBCVAXC^K~BIhBKlBc@lJW|GUzGIlBEjEIzBK`BOhDMxC?JW`HSbFGjBI~CMpEGvCEfDAhAAtFAlGAvG?d@HzHJtGHbGDhB?X@Z@r@BjB?r@@d@BxB@t@BhBHdJF~IAlGCtFE|CAx@IvDOvFOxEEhAQpDSrEEr@y@xMEn@sAhUUvDo@dLMvBaBrYInAmAtRA?Q|CKhBSfDATAVSzCAPOjCqA~T_BtXWjE}@tO_@dGKjBi@lJ_@fGSfDALAREr@QtCQnCMtB]fG]dGKhBa@vGCb@WpEs@jMALq@dL?H}@bSQ~DIhBIfBCf@Cf@IhBANAb@IhBUhFe@bJCj@IpBM|CCl@KjBYdGMfCI~BMpCYdG_@dISbDO|B[pESdCI`Aa@dEIn@]zCUbBY|Be@hD_A`GEV}@~EO~@K`@a@pBQz@CNCLKh@e@tBOj@Qr@_@zAs@tCaBxFsAjEK^cAxCwB`Gc@hAq@`BWn@_AtBc@~@eEtIYf@INsBjDm@bA_@j@qAfBSZGHw@`Aw@`Aw@bAo@v@qDtDcB|AaA|@gBvAOJaDzB]TqAx@gC|AgCtAGBaDhBcCvAsDrBcFpCOLgJdFuFzCeE|BgEzBgCtA}@f@iAl@}C`B{@b@kDjBcB|@eDfB}Az@gAj@OHyBpAGBQJg@VyC|Ag@VcCrAkDhBaFlCMHmFrCsEbCEBeGbDcEzBo@\\\\yF|CC@eG`Dg@X_EvBeAj@[PeCpAIDIDsAt@_@RcB~@_DdBE@gGdDGBeLlGg@VaFhC{EbCsCtA_HdDo@ZgD~AcFdCE@aAf@a@PcAd@IDKDeAh@cAf@_@RoHhD}DjBIFqJrEqKdFgAh@gGxCkAj@yAt@OHOH{BjAWNqGfDa@RC@iHvD_Ad@eDdBsCzAQH_@TeCpAoBdA{EdCcEvBc@VmIjE_@Rg@VcHxDqAr@GDgAh@}C~AmAp@sAr@mDfB{DpBu@`@yBjAiExBiAl@}Ax@g@V}BjAiAl@kBbAyC~Ac@R}Ax@qCzAYNeAh@_Bx@aAh@[NcB|@kDhBs@`@sCxAaAf@SJoAn@_@Ta@R_@RcAf@aAh@aAf@e@ViEzB_EvBC@{BlAgCrAgDhBm@\\\\uBlAaAh@IDyBpA_@VcAj@}@l@aBbAq@d@gBdAg@ZUPQJEDqA|@aC`Bg@\\\\oCnBgCjBwCxByCzBcDjCmDvCiA`AcBvAgDxCg@b@_CvBsApA{@v@}@z@KHKJ]ZYXCB]\\\\sAtA_A`AiFlF_@b@kExE]^{G|H_@b@uF|Gc@j@kCfDeArAi@r@oDzEk@t@o@x@}@nASXaApAgBlCm@~@uArBgCxDiAhB}A~BuEfHIJ{ClEaEhGmHbL}@tAwD`Gy@pA_ChDkC|D_CfDqAlBkBdC[`@qCpDmAxA[`@gEdFkExEiCtCcA`AuDlDy@v@A@o@h@QNON}@v@}@v@wChCIHiDnCeBrAu@j@gDdCiCjBUN}FfEaBjA]T_BhA_C`B_@X_BfA_Ap@YR{FdEeAx@yDtCaBrA_DfCwApAiC|BkAfAmAhAoDlDgBjBs@t@}AdB_BfBiCxCu@~@mAvAwBnCgArAcArAw@dAo@z@U\\\\EFcAzAu@dAu@fAW`@[d@CDMRSZMRg@t@y@rAEHg@x@e@t@QXeAbByAjCKRsA~Bu@rAQ\\\\u@vAMTa@x@Wf@INc@`AKPIPAD{@hBEJ{@fBCD}@fBu@~A_AtBq@|AKT}@tBw@jBiAtCw@pBUj@Sh@EJO^Sj@Uj@Sh@KVIVWn@Qd@i@vAe@pA_BlE]`ASj@c@lA_AhCk@xA_@bA]~@Sj@q@hBaBrEmBjF_BnEM^iAzC{AfEcBzEoAnDi@|AkBvFsAdEK^iBfGyArFkBdHADqAfFQt@s@tCgA~Ee@fBu@zCeAdFOr@kAdF}@zD}AdH_BjHMn@wEhT{@~DMn@_@`B_@bBKj@A@Ml@UdAS~@Op@k@pCyBbKET_DbNg@vBOn@s@dDIZOn@GZmAvFEN]~Am@pCGTEVUfASx@Mp@On@c@lBgA`FOp@c@fBoAdFOn@Qt@qAvEqB~GSl@sBlGIVIRi@vAGNGP?@Sh@Uh@Sj@mA|C[x@o@|AuA|CsBtEsCzFaCpEGJ_DpFQVq@jAqApBW^u@fAwB`DMNoBlCY\\\\wCvDuBdCeAjAiAnAg@h@uBbCuA~AiE`FsB~ByBfCUVuA`BgAnAk@n@[^kN|O[^uA~AsA|AuCzC{A~AsE`EuEdEi@d@qCtB_Ap@_BlAOJMLo@d@_Aj@aEjCaC|AoNbJwClB_BdAaBfA_DtBuBtAmCbBeNxIa@TGDyClBaC|AwH~EmCdBaEhC_@V_@V_@Ta@VOJOJw@f@A?}@p@}HxFgGpEUPcLzJ_D|CwBtBuCtCoAnAwEjFs@|@}LxOA@cDpE{ArBi@|@oDzFeCdEqDfGm@dA_ChEWf@q@lAINKPo@lAq@lAgHrMcCpEcDbGsChFyBbEqBtDa@t@qElIgBtDQ^_DhGsHrP?@mBxE}@zBa@`AcCvGi@|AcE`Mi@~AeApDw@hCu@hCcAhDiBbHAFeBvGmAtF[xA_BfHa@hByAlHu@rDw@vDcD|OwBnKmDvPa@rBuBfKwAdHGZc@vBc@vBI\\\\AHKf@[`B]~A?@gCvLgBrIUdAGZS`AKb@I`@CHMl@ENEPc@zB[`Be@`Cq@bD_C`LUfA_DzOQ~@YpAm@pCERER]`Bk@rC[zAcFhUgDpNQl@o@nC{@fDENELw@`DW`A{ErQiA|DqHjWaBxF}CrKgD`Li@dB}EvPu@zB}CzJmB~FkBtF{DjLkAhDsAxDaCxGgCdHqAnDm@`BaBpEoCrHi@vAwDlKiC`HaE|KcApCgEhL}@bCg@vACFCFELc@hA}@bCoCrHIRaCrGaBdEwAzDk@`BwAzD}AjE]|@KX}@`CUj@ITq@nBi@vAOb@mBnFeEpLaDbJi@zAoEpN_B`FQl@ITIVUv@Qn@Ql@Ob@g@dBSp@iAhDCJe@zAeAlDgBnFsA~DwAbEeCjGk@xAmBpEMX_BnD{@fBq@rAgAtBoBnDEJqCvEKRMRWf@Yd@Yd@Wd@S\\\\EFc@t@KPKPsAxBkAnBuBjDwA~B}KtQAB}FrJyC`F}AfCILyC`FaCzDuCzEqArBmEjHILGJCBsAxB_CzDk@~@mB`DaCzD_CzD_DjFeAdBgFpImApBqCpEGL{C~E_BjCEHuA|Bi@|@qBfDMTcKpPOViGdKOTwExHeC`EgAhByDrGSZq@hAmAnBk@~@{@tAkApB_IpM{E`ImElHs@jAmAnBeBtCYb@m@`A_@l@mApBgFnIoArBw@pAwDlG{AdCcAbBiDtFmD|F_A~AeHnLU^w@nAs@jAOTMT}BxD_L`RiAfBsHpLgC~DwElHeBfCwEzGu@fAoAjBkBlCqAhBa@n@oEpG{IpMaDtEoGhJcClDeCpDgHhKa@l@cBbCmLzPeA|AqC|DORMPmAfBiFzHCFw@nA[b@g@v@aAtAyExGk@v@yBtCaApAaC|C}FtHYZuDjEAByErF[`@[^]^QTy@|@MNGFUVw@|@]\\\\[^]^q@t@GH[^ONaDdD]^C@oCvCsF|FgDnD_KnKw@x@CBy@|@WTUVgBlB_GjGIHwAzA]\\\\STsF`GqBvBoDfE[^[`@]^[^OPORILOR[`@u@fAqG~ImDvFy@vAoFpJa@v@}A`D{D~HcAbCoCpGqDtJu@pBo@jBw@zB{CnJaB|FMd@Of@ADK^Mh@ELGVYdAOr@GPAFq@lC?@i@~BS`A{@xDQx@g@`Ck@dDUhAa@|Bu@dEe@|CU~Ac@tC?@WdBE^_@rCCTCTIj@Kx@QzAAJq@vFI|@OrAo@hGa@~Dk@xFQzAUtBe@rEYhCQfBo@`GGr@_@nDc@zDQ`Bc@pDSfBy@rGi@zDSnAYbBMp@QjACPCNAFKh@Kp@Mr@Kp@Kp@Mp@Ib@gBdJWlAs@dDOp@IXc@vBi@`Cw@|CQl@GZmBrHwBlIOj@a@|As@rCEPGPI`@i@tBa@|A]tAcDzL{BrIcBdG_@nAcCpIADaCzHiC~HeDnJM\\\\eBzEg@tAsCnHm@vAoCxGcEtJ{@hB}AhDUd@GLGNy@dBO\\\\oCvFqCzF_@x@eFlKsBdEiEzI{@`B{A~Ca@x@{@fBQ^kBxDiA~BeCdFq@rAm@pAWj@Ub@]r@i@fAi@fAcArB{@bBiB~Do@pAmAjCMVWh@MX]r@Wj@Wj@oBjEq@pAyAxCsCzFeAvBqBfEYl@eIrPyBpEeD|GCFyAxCOXOXeAxBcD~GS`@uEtJkFxKcGxLa@t@_@t@sArCuGhNq@rAwBfEmFrKm@lAwChFa@r@uBpD}B~DuAbC}AbCaCzDaDhFgAbBmAlBUZU\\\\s@fA]f@W^{HxKwBxCuDhFuBvC_ErFILoG~Ik@v@mBjCwC`EyBzCoElGuBxCgCnDiBhCABw@dAuLlPmH`KiDxEaIxKyCbEe@n@mBjCu@dAoBnCY\\\\mBhC{BxCeElF{CrDuA`ByBbCmCtCkDrD[^KJC@wAzAyAzA]\\\\oIpIUVkJjJWVuCvCiBjBSTCBYZ[^[`@[`@]^mCfDoAfBqAhB[b@yAtBW^QXSVU^{AbCi@|@{@xAw@pA_BrCkArBKVg@bACFa@z@eAzB}AhDiAdCcAdCeBjEy@zBi@xAq@hBq@tBm@fBk@lB}@xC?@q@zB_@xAEPu@nCa@xA_@`Bg@pBm@lCs@|Cg@|Bk@fCe@vBWjAQt@k@lCOn@_@~A?BS|@Op@YtAOt@YnAa@dBe@pBe@vBgArE?@i@nB_@xAw@vCoAhEg@`BGLoAxDq@xB{@~Bi@xAaAlCy@nBc@fAu@hBgCzFmAjCe@~@gAxBIPe@|@INGLS^OVEHGJaCjE{BpDwBnDkBlCy@fAoD|EwAhBsA|AeBrBKL{@`AMNIJIHa@b@_C|Bc@b@URKLuCfCIFy@p@c@^}@r@e@^iBtAu@n@{@l@e@\\\\a@XGFu@j@iAt@MHm@b@eAn@c@XiAn@o@Z}Ax@KFs@^oAp@aAb@c@R]Ni@TqBx@kAh@y@Xg@POFwAh@MDqA`@UHi@NkA\\\\UFy@TeAVKBc@La@JgAV{A\\\\s@N]Fs@NyBd@uE|@YFI@a@HaAPg@NA?a@HWDsB`@QDoB^mCl@qAXu@P_B^gAZw@VkAZ[JODMDQFcAZQFsCbAaBl@[Ng@RuAj@q@\\\\_Br@o@Z}Av@WL]NcAj@c@ViAr@w@d@C@u@b@y@h@o@`@m@`@sA`Am@^k@d@g@\\\\m@d@KHa@ZuAhAu@n@_At@u@p@qBjBkAhAcAdAA@kAjAs@x@SRu@v@_AfAsAzAcBnBORa@d@eApAs@x@OP}AlBiApA{@dAONsAbBgApAy@`AiAtAcAlASTaAjAkAxAGHwAbBoB`CwF|GmG|G}A|AyAxAaBbBMLMLcBbBGFuAjAqK~IA@eIdGu@h@a@VuGlEKFiH|D_@TgCtAcD~Aa@ReClAo@XuAp@a@Ra@Py@`@yIdEsI`EaDdBgGfDgE|BaAh@kKpG[Ra@TcEfCiC`ByA|@_@TaAl@aCxA[RgBdAkAr@uAz@a@TmElCwA|@yClBeGpDGDaAl@aAj@MHOHaAl@aBbA{@f@wG`EuBnAaCxAKHUL{A~@cDnBeI`FqDxBs@b@aCxAcEfCeHlEcGtDgHlE_Ah@eGrDQLqEnCeF|CaDnBmFbDyBrAcEfCuCdBm@`@cCxA_Al@cCxA_BdAcGvDeJ~F{CdCyGpFgA|@s@n@GHIFeD~CcCnCyAbBeE~E}AjBgF~GKLY^i@r@eCjD{B|CsBvCoCnDsBtCe@n@mBjCA@iEpFmAxAGHgCvCc@h@oArAy@x@}@~@m@n@eCbCm@j@_DxC{@x@u@r@m@n@s@n@q@p@m@j@EDw@r@i@f@i@h@o@l@q@p@]X[\\\\KHs@r@g@d@QPKJ]\\\\EDcDxCURaCzByAvA{AvAyAtAsAnAqEjFuA`B[^gEdFeBrB_AxAyBhDc@p@iBrCiAdB{LrUUh@{CbHYj@MXyB`FO`@mCdGsG~NQ`@gGpNkCdG}CfHs@~AGNm@vAEH{AhD}CdH}CdHoBrE_AtBiB`EKTyAfDqAvCu@dBs@`B}@nBwAbDeA`CyAfD{AlDCDc@`AwAfDgAdCkBjEgCbGaAxBqAzC_BtDq@|AuA~CuBxES\\\\EH{CzG{CdHcA`C{CdHcAzBUj@cFbLgAdCiIfRCFwFnMO\\\\O\\\\qBrESf@yC~GgAfCwEpKSd@iBfES`@sA|CsA|CuA~CQ`@gEvJiJbTsBzEu@fB{BpFe@hA_BdEeArCKZs@lBq@hByBtGiAlD}@nCuAvEoBfH{AxFcBzGe@lB]vAa@hBEP_@|AUhACFGZETIXa@jBQ|@e@bCCNg@fCg@hCo@dDm@pDo@bEWxAKt@eA|Gu@rF[bC[fCMdA]zCE\\\\MhAWvBMxACNk@rF?@YzCALw@dJMdBMdB[nEMrBGx@UbEG`AMhCS~ESpEMvDQjFMbF?NC~AAJEfBAx@A`@?RAt@CbAErC?DCdBC`BGtFGbE?JYlUGlFE|BCdCGvFGxECfDCtAKbI]hXEbDGvEAt@?NCxBAlAAhAA`@GxFMvIArACpCGlEE~CA^CzBC`BClCEbDClCCjBGvEAv@E`DCjBOnLKbIg@x`@_@v[E`DGnD?^E`DAt@GvEE`Ds@rl@GvEGjE?JAt@Ar@ClB?DGzGANQjOS`RQzOIrGAv@GfGCvAEhDCpDCp@EdDKvFMfGQlFEv@K~CABIfBGrAATEt@UtEEt@QhCIhAK|AM~AQxBSzBUjCSlBAJO`BYjCi@zE]nC[zBo@zEQjAW|A]zBc@jCY`BUpAe@dCYzA[bBc@tBe@tB{@xDk@bCq@tCa@|A_@xASp@c@`Bi@jBQp@Qn@?@c@vAe@zAo@rBq@tBw@~Bm@jBq@lBu@xBs@pBGPw@xB_AdCQb@e@rAs@hBe@fASf@w@rBw@lB{@rBs@bBe@bAi@rA[p@Sb@INc@fAA@_ArBmAhC]x@c@|@g@~@yAzC_AdBEJi@`AWb@Yj@ILGLaAjB_@r@g@~@kApBa@t@k@bAuBlDQVs@hAo@dAu@vAGNGLKNi@x@s@fAMR}A|BMPoAlBe@t@q@~@uApBILeAvAw@hA]b@oAfBc@h@wApBMPu@dAoAdB]f@i@r@KP[`@UXw@hAMRGHGFGLEDy@hAEDi@v@KLKNCDCBCBk@t@iAzAMPg@r@o@z@EHGDED?@EJY`@cAtAu@bAUXo@z@iA|AORSX{@lA]b@ORe@p@[`@qAhBSXc@j@q@|@wCdEaB~Bk@v@_AtAMPYb@iAdB_@j@}@tAsBjDiApBw@rA_@p@iAvBcAnBwBnEeA|BsAvCADiCfG_A`CQb@{@zByA|DuA`E}ApEu@bCe@xA{B`He@xAs@xBIVo@pBGPq@tBSl@k@fB_@nA{@fCM`@yB|GOd@aBjF{AxEy@hCCHk@hBqA~DgAhDa@pAmB`Gg@~A_AxC_B|EmBtFaB~Ea@dAq@hBq@fBCHUh@g@pAo@zA[t@e@hAiAjCqDtIcDzGiCfFeAtB_AfBgAnBwFtJm@bA{AbC}AbCeCrDs@dAiA`B}B~CiAzAMNeB~BaHlIcEpE[^kE|E{@z@]^uCvCa@`@oBvBkFxFwC`Du@v@}WbYoBtBmAnAkAnAuBzBsCxCmExEaJrJkFvFuBzBWVUVuBzB]\\\\cItIwAzAoCtCQRQRcBfBEDaAbAkAnAg@h@mApAaJtJ}JrKwAzAsGhHm@r@mEhFs@|@uClDq@x@mB`CuAhBq@z@wGxIwDjFe@t@g@v@GFiBpCaAxAmFfI_F~HgKlP}DnGILm@~@}AdCeGtJiCbEQZgAfB_@l@mAnBmAnBoEhH{LzRiJdOMPKRqEhHe@v@GFQX}E`IiFrIs@fAgBrC}C~EsJpOUZuCfEcDrE_C~CeC`DEFs@x@uAbB[^g@l@m@r@y@|@uB~BGH_B`Bk@l@aAbAqBnBWVoBnBwBnBmAfAsAlAeCpBCB_DdCwB|A_CfB}B`B_ErCmJzGqBvAaDvB_BfA_BhA_BhAaErC}@n@aErCk@`@sE`DaItF_MvI_C`BmCjByAbAoBzAGDoDrCiA`AiCxBkDzC]\\\\oCfCaBbB{@z@sCxCWXkCvCKJqB|By@bAyBlCGFEFkCfDGH[`@{C`E_AtAkBnCYb@W^sApBs@fAk@z@aE`HoDjGm@fAaEtHADmCtFMVsDhHMXILoBzDuDrHMXcHfN?@_HbN_EbIiAvBi@fAq@rAa@t@Q^qCpF_AjByAzC}@bBu@xAcAlBS`@m@hA}@|AsAbCKPaBtC[h@iAlBw@nAWd@W`@eC|Ds@hACBU`@yBdDKNeBjCEDo@~@EDo@~@i@v@kA~AoAbBCDW\\\\sAdBKNMRUZk@r@SVKNMNA@mA|A{FpHgDjEkChDgDjEsAdByGrIgDjEOR[b@cApAQTaAnA}@jAaDbEcElFuHtJgDjEcElFw@bAw@bAMNKNmCjD_FnGsCtD}AnBe@l@cC`Dc@h@kChDyD~Ek@r@uBpCKJUVk@t@aAlAKLc@h@_@d@CBkAxAw@|@}ApBABkCdDEFORa@j@a@h@k@x@]f@o@~@uAtBWd@w@lAo@fAYf@QZkBdD{@zAaAdBsA`COVcB|Ce@x@oA|BGHiAvBm@jAm@nAYn@CBc@bA}@pBg@nAUn@MZKVSl@c@hAUp@e@tAM`@Y~@Sp@e@zACHw@|CADw@hDe@tBw@~Du@zDk@|CMn@YxAMr@ADYzAQ~@SfAaAnFmBnK_@vB]rB_@dCQpAWdBSrAIl@AFGf@Kr@Iv@Kp@Q`BKx@AHKv@Ip@KhAM`AEf@WbCEh@YzCALOhBIz@UbDIlAIdAEb@GfAIlAG`AEdAC^G`AGfAGrAG`BEfAGjACx@EvAARE~@CvAEbACpA?@Ct@?REbBAz@CfBC`AAhCGrM?`A?`@?bABvD?p@@lB@b@@|B@t@BlC@jA@dB@|@DdDBlDDzCB~CBxCDzDD`EBlDBfCBnC@d@@bBBrB?RDbD@vA@j@BbD@~@BnCB~B@t@?RBbDDhEBdEDnD?v@?`CBxC?fD?jDAzDAbDAlDCfDA|A?v@At@AlACtBCzBCpBAd@CzBA^ATClBE|BAb@EjBEvBCj@A`AE~AAJGzBGtBC|@GhBE~@GvBGrAG~AEdAC|@Cj@GfAGrACp@GvAEv@GrAEr@Eh@?NALOtCGhAUdEYdE[vE[`FGp@Q|BYzDa@xEa@vEUlC[hDg@bFWjC_@jDo@|F[tCCN[lCQtAKt@Gj@CTALId@QpAWnBQrAE^Kt@[|B[tBS~ACPQbAG^M|@c@nCa@jCO~@e@xCMn@a@hCYvAEXKf@]zBe@jCWtAs@tDi@pCq@hDk@rC[vAe@zBMp@GXmArFu@jDOl@gAxE}@lDi@|Bk@~Bq@hCcBpGkBzG]nAyBvHEJsD|Lw@|BiC|HmAtDcAvCa@fAqApD_AdCqAnDcFhNwDhK_AjC_HpR_AhCcFbN_CrGe@lAgB|EaCtGaClGmB~EuBvE}BvECDgCxEqB`DkBnCEDeBbCkBzBIH}AlB_B`BoAnA}BzBoB`Be@^uBfB}@v@mCrBo@f@cAz@eBvAiBzACB_CnBqB`BeAz@y@p@eBvA{@n@]VuAdAcBbAIF_@Vo@`@sAt@MF_@Rg@VqAh@QHsAh@UJaBh@QFsA^w@Pg@LyAZqAVE@wAXkAVmARa@J}A\\\\cBZu@Pu@NsA`@C@}@XuAh@_Ab@WLiAl@y@h@CBa@R{@j@YPgBlAMFsAt@ULk@ZaAh@A?}@`@CB_@N_@PYJi@T{@\\\\OFiA^_AZiC|@WHi@TcAZi@R{Bt@yAf@a@NOF}Af@MFm@R_A^{Af@IBk@RoA`@?@kDjA}Bv@c@Lg@PsAd@o@RyAf@IBgA`@k@R{@Z_AZyAf@A@eAZwAd@}@ZoFnBa@NMDSHgDbAqGzByGbCkA`@aEtBKDuA|@cCzA]XmDlCo@d@aD|B}@r@SNmBvAuCtBED_Ap@sAbAiAx@eAt@sAbAkBrAwB~A_BlAy@l@uB|Ao@d@IFWR]RQJMJC@_@Ts@d@k@ZQL{@d@SLc@V{@h@y@j@w@j@oA~@_Av@iAbAONm@j@kAbA}@x@_@Zm@d@o@h@EB{@p@u@f@GDw@f@UNEDEB]Ta@Xu@h@a@ZWPGDKHi@b@i@^_@XA@]T_@X_@X_@V_Ap@OLQLu@p@MJ}@t@CB}AzAcAbAg@j@_@b@MNo@v@_AfAcApAs@bAy@nAs@dAo@dAm@dAKRq@jAy@`Bq@rAMTUf@q@~A]v@ITi@tAw@rBo@lBg@~Ac@tAGV[dAELSl@K`@EJEPK\\\\Sn@CLSr@ELY|@EPAB[fAADOh@Qh@]hAGVY`AKZ]hAg@dBi@jBk@lBIX]jAm@vBOd@Qn@Ql@GPQj@]jA?@a@rAAHSl@AD]dA[z@AB]|@_@`AUf@EJKRYr@a@x@IPWh@]p@a@x@GJ]l@e@z@a@p@EFc@p@KL]h@_@h@Y`@SVML[b@k@n@]`@IJg@j@GF_@b@a@f@g@h@c@f@_@b@EFo@r@u@z@q@v@w@~@QPk@p@MLs@z@a@d@MLOPg@l@GHe@h@[^y@~@[\\\\A@wA|AKJo@p@QRKH]Z]ZQNMJ]ZURIF]Z_@Z]ZQPKJ[VCB]Z{@x@}@x@eA`AqAnAy@x@]\\\\]\\\\{@|@MLOL_@Z]Xg@b@C@SPSNEBy@n@s@h@a@\\\\e@`@[XC@{@x@]\\\\_@ZkAfAMNo@j@gAjAq@p@IJw@~@?@_@f@e@p@UZa@p@IJa@r@i@|@CDk@~@[n@Ub@o@rAo@rACDi@nAQ`@m@~AUj@Sj@EHOb@Wt@Sp@K`@CFQl@_@tACJOn@On@ADOp@Qn@UdAWlACLUlAAFSlAEVc@xCIp@CRCPAF_AbIWnCIjAEn@EfACh@CnAC|@?HEpB?J?B?JAn@?T?`A?d@?j@?fA@t@?n@?TBpABnADtAD~@DlA@TBj@FdAHfAJfA@HHr@@NHp@Df@@PBPHr@Hr@@FHj@@LFd@BPLjAJ~@Fr@B^HbAHhAFz@B`@Bb@DdA?@FnADlADlA?`@@`@BfA?@@p@?|@?r@?R?`@?X?Z?p@?DA|@AR?P?PAb@AHAlAATCp@ElAElAGdAE|@YzDUjCShBQnAIn@Kv@ObAStAKp@EVGZWdBUrACPYdBIh@i@hDWvAOt@Or@Mh@St@Qr@EJCJW`Ak@nBIT]dA[`As@zBOf@g@xAQf@Wp@]|@MVUh@Wh@ITKRYf@Wf@KRMPCDCF[h@GLWf@Wf@Yf@GJMZWd@m@vA?@k@rAo@fBM\\\\o@nBY|@c@~ACFMb@g@dBWbAc@zAk@jB[z@ENq@fBELg@lACFCHEFm@rAQ\\\\u@xAMT}@|AaA~A_AvA}@nAeApAuA|AqApAmAhAaAv@m@f@kA`AeAv@aBdAcBz@qAj@_Bf@WHYFIB{@P_@HMBi@Hg@HS@g@Dy@Fc@@Y@W?S?i@Ek@CA?y@Kg@Go@Mc@Me@Mg@Ow@Ua@QWKe@OA?aAc@_@Q]Oc@Mc@O_@Ms@c@_Ak@oAw@]UgBiAg@YUK[Oc@Qi@Ss@Sk@Mm@K[EYEo@G]C}@CS?K?K?k@?[?c@?}@@G?m@?m@@sA?i@?i@@_@?eA?iA@e@@]?A?c@Bg@@gAHgCPy@DO?I?}@@uAESAc@AYAqAE]Ai@?M?U?g@?iB@cA@aA@G?s@@sC?sEBuEBmA@iB@uABqAFaADs@DQ@oBL_@Bc@DA@eALeALI@YFuAVSDyAXyCf@wCd@}FbAyBZwCd@eAPaC^mATaBXo@HkAPyB^yAX}B^KBwCb@{Ex@mDj@w@LyB\\\\_HjAq@NiB`@o@LsAXk@LMDuDr@YHkDx@uAZWDg@HcAR{A\\\\mB`@{@RiAV_ARaAToA\\\\aAZ_A^}An@EBa@RYNeAh@kAr@mAr@eB`AgB~@{@^sAf@mA`@a@N[Fc@NqBf@_APMBiBTm@Ho@HgAPo@J[FWDi@LIBy@V_@Je@Pw@VgA^aBh@[J{A^eB`@wCd@aAPqCf@aCf@oBb@iBb@a@JiAZmA\\\\_AV]Jm@RgAZuA`@sA^wAb@y@VaBf@_AVGBgAZa@L]L_AVIBC@C@g@Ni@PUF[Jm@PUFKDc@Lm@PWHQFk@Pg@NMDYHaAX_@L[JiAXEB}@Rw@NG@g@Hq@Fq@FwAFgADe@@O@iABo@Bs@Dm@Fq@HOBe@Jg@L]Ho@To@Ty@b@q@\\\\MJcAl@i@`@i@\\\\IHi@^s@f@eAx@[RSNq@f@i@\\\\u@f@[PQHOFmAh@cBl@sAb@mAZC@sAZy@Pg@L}A^g@Nc@LIBiAZu@VSHq@RmA`@{@VA@}@XgAX[Lg@La@Ng@Nc@Na@Pg@Ta@RWPA?]PUNSLu@j@]VYV_@\\\\WTML]`@a@b@_@h@c@j@W`@[b@IN[b@i@v@[d@c@r@CBU^_@l@U\\\\QVu@dA_@b@u@~@[Ze@f@CBe@f@s@t@[ZOJm@l@GFe@`@OLg@b@u@h@A@]TMJQJq@b@g@\\\\MFKHOHo@`@YNCBsAx@[RIDC@{@j@k@\\\\SLQJEB_@TUPC@c@XIDKH[TA@a@Xg@^YRQLe@`@GDWVe@d@URi@h@q@v@m@r@y@bAa@j@QTcAtAo@~@]f@i@|@c@t@INo@lAm@jAs@tA]t@Sf@Wn@k@tAe@nAe@vA]jAc@`BYfA_@tAYhAi@fBe@xASj@Ul@[x@]x@]x@_@t@e@`Ac@v@[j@Yd@[d@e@p@o@z@_@h@[`@W\\\\Y^a@j@e@l@s@|@c@d@o@n@e@`@QJe@ZYPc@Ti@Tq@Tu@ReAZ]LYHc@Rc@R_@T]VWPm@f@g@`@m@t@c@l@[b@S\\\\OT]l@Wb@i@dA]r@a@v@y@bB[p@]v@y@jB{@jBKT_@r@o@bAY`@W\\\\y@~@_AdA_AbA}@`AqB|BYZWZUV[`@UZ]f@Yd@Wd@Yh@o@rAa@~@i@rAi@xA?@Sd@Qd@e@`Ag@hAEHYl@?BMVO\\\\M^K\\\\Md@IZAHYlAAH[xAWfAOt@Op@_@bBOh@GXQl@GNQf@Yv@Yp@a@dAQp@Kf@Sl@IRYlAWhAAFGXQ|@EJMj@Ib@K^K\\\\MZKZKZ[r@[p@]t@Yn@CDYn@[r@MXEJmBdEgBzD}@S\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 47.5851,\n                        \"lng\" : 6.89798\n                     },\n                     \"transit_details\" : \n                     {\n                        \"arrival_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 48.84494,\n                              \"lng\" : 2.37348\n                           },\n                           \"name\" : \"Gare de Lyon\"\n                        },\n                        \"arrival_time\" : \n                        {\n                           \"text\" : \"8:42 AM\",\n                           \"time_zone\" : \"Europe/Paris\",\n                           \"value\" : 1728715320\n                        },\n                        \"departure_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 47.5851,\n                              \"lng\" : 6.89798\n                           },\n                           \"name\" : \"Belfort - Montbéliard\"\n                        },\n                        \"departure_time\" : \n                        {\n                           \"text\" : \"6:05 AM\",\n                           \"time_zone\" : \"Europe/Paris\",\n                           \"value\" : 1728705900\n                        },\n                        \"headsign\" : \"6700\",\n                        \"line\" : \n                        {\n                           \"agencies\" : \n                           [\n                              {\n                                 \"name\" : \"SNCF Voyageurs\",\n                                 \"phone\" : \"011 33 1 84 94 36 35\",\n                                 \"url\" : \"https://www.sncf.com/fr\"\n                              }\n                           ],\n                           \"color\" : \"#da001b\",\n                           \"name\" : \"Paris - Mulhouse Via Besançon TGV\",\n                           \"short_name\" : \"TGV INOUI\",\n                           \"text_color\" : \"#ffffff\",\n                           \"vehicle\" : \n                           {\n                              \"icon\" : \"//maps.gstatic.com/mapfiles/transit/iw2/6/rail2.png\",\n                              \"name\" : \"Train\",\n                              \"type\" : \"HEAVY_RAIL\"\n                           }\n                        },\n                        \"num_stops\" : 3\n                     },\n                     \"travel_mode\" : \"TRANSIT\"\n                  }\n               ],\n               \"traffic_speed_entry\" : [],\n               \"via_waypoint\" : []\n            }\n         ],\n         \"overview_polyline\" : \n         {\n            \"points\" : \"sestGcydw@yz@_{@ycAbQmgEgcDqzDyyDkkAaX_k@r}@avCpiFudCjlAa_B`nB_m@xnHkyDdoCmqBjvC{vCnXklBjcAsvA{L}aBpiAw}As}Bq|DbbG}gCvjGeLhiB{t@v|Aas@|a@{l@sv@_mBnNo|BoYykC`g@obBxz@oi@xkA_{Axa@_i@a`@yuA|RiGd}@mw@sYi_AtDwpAb{@qTqAygFiiBq{JohFwgFfRkf@{n@efActBot@c_Ae}B{pB_{BOmiBhbBc_D`fAceIxbEo_Dvk@qdAbdBagAheEofo@jw]omj@~qWg~GzcBocCnRu|Inp@mhDpd@aaCwgAooAg}@wl@|s@crBz`D|Fp}Hsy@bxEudC~bEewAlcC~QxrA|`DvdGr_D`eG}bBhnEwEv`DzaAfQmBm|@hRf_@quCdaC_lAll@y|@s\\\\b]lsBen@r}BwzAfk@eaC|jNkvFb}IozA~cGgJbqIsLj{CkjBrj@osCqSumKx}EysEphF{dD{ZezBcp@w|Ln`DgxBfgDgRdtCm^dzMceKhqTg`AxxGoUrzAvu@qfBxaEuYt~FzbAbgAmC`mAnr@Vx~Elm@`yD|eC~cEhaA|aEbA~sC`j@zuB~EnrDjm@bnExjAdGdpAhbBf}CtwHiA~nJd@rlD_zA|jE_bB|a@sMr{BmrEdxDwjAsYqRzkG{kBlbAcl@~uAckDtv@sjBlrDatA``@ab@f]qiEvmEsCbQuLvl@wUt~@eQjgAyJvz@qo@`l@ixAiPczAzpB~DjdBjnClzMlmAnog@xIziU`{DxePdpCxdMzVb`LfpEl|Jv_E~{S`pDphFh~FjmFpsDhtDzcCdbHbpCl`WpjBrhKj`ArjKhmAtyIbc@xyK|{@lhQtSngOpq@t~J_ErtLmhHjpSceClaNcvAz|I_gCrb@cx@ndFkq@xdG`gBpaElDd~FgiArpAvZhqCt[joCa{@zaDemFpyHw~C~aBgqEl{Fa|Az`DirDtqB{rFrjJuuE`pOjc@n}DuaBzjDwuG`{FwkBpqG~B~xDq_BjlC{jA~|HysAv}Pwh@fhJi{Cl}DwxLzlG}_KpzJe}DdxKcqBvdEkgGpuFc}HthWwzI`rPsdJlsNmaEdvNehKfpPc~AndEqhBl|@{iItwFkgG`nEa`DbiEosDdpIstApvG_q@h|TcwCxrFyrBbkFidGzoHy_MdmOmyGx`K_h@hyJggB`xMwcDzkEinBrs@yvEbxDsgBduCol@v`EceBdnEcnF`D}iFdrAojGbfHqYdt@\"\n         },\n         \"summary\" : \"\",\n         \"warnings\" : \n         [\n            \"Walking directions are in beta. Use caution – This route may be missing sidewalks or pedestrian paths.\"\n         ],\n         \"waypoint_order\" : []\n      }\n   ],\n   \"status\" : \"OK\"\n}\n")

	segments, err := decodeDirectionsTransit(body, originCity, destinationCity, "train", isOutbound)
	if err != nil {
		log.Println("error decoding JSON: ", err)
		return nil, err
	}

	return segments, nil
}

func GetDirectionsBus(originName, destinationName string, originLatitude, originLongitude, destinationLatitude, destinationLongitude float64, date, hour time.Time, isOutbound bool) ([]model.Segment, error) {
	// get cities (with or without iata)
	originCity, err := GetCityNoIata(originName, originLatitude, originLongitude, false)
	if err != nil {
		return nil, err
	}
	destinationCity, err := GetCityNoIata(destinationName, destinationLatitude, destinationLongitude, false)
	if err != nil {
		return nil, err
	}

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading the body: ", err)
		return nil, err
	}

	// check response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("errore HTTP: %d - %s", resp.StatusCode, string(body))
	}

	// TODO remove this line and uncomment the api call
	//body := []byte("{\n   \"geocoded_waypoints\" : \n   [\n      {\n         \"geocoder_status\" : \"OK\",\n         \"place_id\" : \"ChIJ53USP0nBhkcRjQ50xhPN_zw\",\n         \"types\" : \n         [\n            \"locality\",\n            \"political\"\n         ]\n      },\n      {\n         \"geocoder_status\" : \"OK\",\n         \"place_id\" : \"ChIJD7fiBh9u5kcRYJSMaMOCCwQ\",\n         \"types\" : \n         [\n            \"locality\",\n            \"political\"\n         ]\n      }\n   ],\n   \"routes\" : \n   [\n      {\n         \"bounds\" : \n         {\n            \"northeast\" : \n            {\n               \"lat\" : 48.835687,\n               \"lng\" : 9.1271694\n            },\n            \"southwest\" : \n            {\n               \"lat\" : 45.4895006,\n               \"lng\" : 2.3801607\n            }\n         },\n         \"copyrights\" : \"Map data ©2024 GeoBasis-DE/BKG (©2009), Google\",\n         \"legs\" : \n         [\n            {\n               \"arrival_time\" : \n               {\n                  \"text\" : \"6:10 AM\",\n                  \"time_zone\" : \"Europe/Paris\",\n                  \"value\" : 1728965400\n               },\n               \"departure_time\" : \n               {\n                  \"text\" : \"5:30 PM\",\n                  \"time_zone\" : \"Europe/Rome\",\n                  \"value\" : 1728919800\n               },\n               \"distance\" : \n               {\n                  \"text\" : \"651 km\",\n                  \"value\" : 650799\n               },\n               \"duration\" : \n               {\n                  \"text\" : \"12 hours 40 mins\",\n                  \"value\" : 45600\n               },\n               \"end_address\" : \"Paris, France\",\n               \"end_location\" : \n               {\n                  \"lat\" : 48.8356869,\n                  \"lng\" : 2.3801607\n               },\n               \"start_address\" : \"Milan, Metropolitan City of Milan, Italy\",\n               \"start_location\" : \n               {\n                  \"lat\" : 45.4895006,\n                  \"lng\" : 9.1271694\n               },\n               \"steps\" : \n               [\n                  {\n                     \"distance\" : \n                     {\n                        \"text\" : \"651 km\",\n                        \"value\" : 650799\n                     },\n                     \"duration\" : \n                     {\n                        \"text\" : \"12 hours 40 mins\",\n                        \"value\" : 45600\n                     },\n                     \"end_location\" : \n                     {\n                        \"lat\" : 48.8356869,\n                        \"lng\" : 2.3801607\n                     },\n                     \"html_instructions\" : \"Bus towards Paris - Bercy Seine\",\n                     \"polyline\" : \n                     {\n                        \"points\" : \"ktstGysuv@_lkCrbeQuc`Odt~U\"\n                     },\n                     \"start_location\" : \n                     {\n                        \"lat\" : 45.4895006,\n                        \"lng\" : 9.1271694\n                     },\n                     \"transit_details\" : \n                     {\n                        \"arrival_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 48.8356869,\n                              \"lng\" : 2.3801607\n                           },\n                           \"name\" : \"Paris City Centre - Bercy Seine\"\n                        },\n                        \"arrival_time\" : \n                        {\n                           \"text\" : \"6:10 AM\",\n                           \"time_zone\" : \"Europe/Paris\",\n                           \"value\" : 1728965400\n                        },\n                        \"departure_stop\" : \n                        {\n                           \"location\" : \n                           {\n                              \"lat\" : 45.4895006,\n                              \"lng\" : 9.1271694\n                           },\n                           \"name\" : \"Milan Lampugnano\"\n                        },\n                        \"departure_time\" : \n                        {\n                           \"text\" : \"5:30 PM\",\n                           \"time_zone\" : \"Europe/Rome\",\n                           \"value\" : 1728919800\n                        },\n                        \"headsign\" : \"Paris - Bercy Seine\",\n                        \"line\" : \n                        {\n                           \"agencies\" : \n                           [\n                              {\n                                 \"name\" : \"BlaBlaBus\",\n                                 \"phone\" : \"011 33 9 69 32 33 48\",\n                                 \"url\" : \"https://www.blablacar.co.uk/bus?comuto_cmkt=UK_GMAPS-PRO_ALL_ALL&utm_medium=Partnership&utm_source=Gmaps&utm_campaign=UK_GMAPS-PRO_ALL_ALL\"\n                              }\n                           ],\n                           \"color\" : \"#f25455\",\n                           \"name\" : \"Paris City Centre - Bercy Seine \\u003e Geneva - Bus station \\u003e Milan\",\n                           \"short_name\" : \"BlaBlaCar Bus\",\n                           \"text_color\" : \"#ffffff\",\n                           \"vehicle\" : \n                           {\n                              \"icon\" : \"//maps.gstatic.com/mapfiles/transit/iw2/6/bus2.png\",\n                              \"name\" : \"Bus\",\n                              \"type\" : \"BUS\"\n                           }\n                        },\n                        \"num_stops\" : 2,\n                        \"trip_short_name\" : \"5710\"\n                     },\n                     \"travel_mode\" : \"TRANSIT\"\n                  }\n               ],\n               \"traffic_speed_entry\" : [],\n               \"via_waypoint\" : []\n            }\n         ],\n         \"overview_polyline\" : \n         {\n            \"points\" : \"ktstGysuv@_lkCrbeQuc`Odt~U\"\n         },\n         \"summary\" : \"\",\n         \"warnings\" : [],\n         \"waypoint_order\" : []\n      }\n   ],\n   \"status\" : \"OK\"\n}\n")

	segments, err := decodeDirectionsTransit(body, originCity, destinationCity, "bus", isOutbound)
	if err != nil {
		log.Println("error decoding JSON: ", err)
		return nil, err
	}

	return segments, nil
}

func decodeDirectionsTransit(body []byte, originCity, destinationCity model.City, transitMode string, isOutbound bool) ([]model.Segment, error) {
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
				Date:               time.Unix(0, 0),
				Hour:               time.Unix(0, 0),
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

			stepDepCity, err1 := GetCityNoIata(step.TransitDetails.DepartureStop.Name, step.TransitDetails.DepartureStop.Location.Latitude, step.TransitDetails.DepartureStop.Location.Longitude, true)
			if err1 != nil {
				return nil, err1
			}
			stepDestCity, err1 := GetCityNoIata(step.TransitDetails.ArrivalStop.Name, step.TransitDetails.ArrivalStop.Location.Latitude, step.TransitDetails.ArrivalStop.Location.Longitude, true)
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

			// create segment
			segment = model.Segment{
				// id is autoincrement
				DepartureId:        stepDepCity.CityID,
				DestinationId:      stepDestCity.CityID,
				DepartureCity:      stepDepCity.CityName,
				DepartureCountry:   departureCountry,
				DestinationCity:    stepDestCity.CityName,
				DestinationCountry: destinationCountry,
				Date:               time.Date(returnedTime.Year(), returnedTime.Month(), returnedTime.Day(), returnedTime.Hour(), returnedTime.Minute(), returnedTime.Second(), returnedTime.Nanosecond(), returnedTime.Location()),
				Hour:               time.Date(returnedTime.Year(), returnedTime.Month(), returnedTime.Day(), returnedTime.Hour(), returnedTime.Minute(), returnedTime.Second(), returnedTime.Nanosecond(), returnedTime.Location()),
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

	return segments, nil
}

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
					Date:               time.Unix(0, 0),
					Hour:               time.Unix(0, 0),
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
			// set departure and departure id
			if segments[i].NumSegment == 1 {
				segments[i].DepartureId = originCity.CityID
				segments[i].DepartureCity = originCity.CityName
				segments[i].DepartureCountry = ""
				if originCity.CountryName != nil {
					segments[i].DepartureCountry = *originCity.CountryName
				}
			} else {
				segments[i].DepartureId = segments[i-1].DestinationId
				segments[i].DepartureCity = segments[i-1].DestinationCity
				segments[i].DepartureCountry = segments[i-1].DestinationCountry
			}

			// set destination and destination id
			if segments[i].NumSegment == len(segments) {
				segments[i].DestinationId = destinationCity.CityID
				segments[i].DestinationCity = destinationCity.CityName
				segments[i].DestinationCountry = ""
				if destinationCity.CountryName != nil {
					segments[i].DestinationCountry = *destinationCity.CountryName
				}
			} else {
				segments[i].DestinationId = segments[i+1].DepartureId
				segments[i].DestinationCity = segments[i+1].DepartureCity
				segments[i].DestinationCountry = segments[i+1].DepartureCountry
			}

			// compute date and hour
			if segments[i].NumSegment == len(segments) {
				// if last segment, based on previous segment (if present)
				if segments[i].NumSegment > 1 {
					prevTime := segments[i-1].Hour
					prevDuration := segments[i-1].Duration
					walkDepTime := prevTime.Add(prevDuration)
					segments[i].Date = walkDepTime
					segments[i].Hour = walkDepTime
				}
			} else {
				// if not the last segment, based on the next segment
				nextTime := segments[i+1].Hour
				walkDuration := segments[i].Duration
				walkDepTime := nextTime.Add(-walkDuration)
				segments[i].Date = walkDepTime
				segments[i].Hour = walkDepTime
			}
		}
		// else don't modify
	}
	return segments
}

func GetCityNoIata(name string, latitude, longitude float64, exact bool) (model.City, error) {
	cityDAO := db.NewCityDAO(db.GetDB())

	// check if a city with same name exists
	city, err := cityDAO.GetCityByName(name, latitude, longitude)
	if err == nil {
		return city, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.City{}, err
	}

	// if not exact
	if !exact {
		// check if a city with nearly same coordinates exists
		deltaCoordinates := 0.3
		city, err = cityDAO.GetCityByCoordinates(latitude, longitude, deltaCoordinates)
		if err == nil {
			return city, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.City{}, err
		}
	}

	// create a city without iata
	city = model.City{
		// id autogenerated
		CityName:    name,
		CountryName: nil,
		CountryCode: nil,
		Iata:        nil,
		Latitude:    latitude,
		Longitude:   longitude,
	}

	err = cityDAO.CreateCity(&city)
	if err != nil {
		return model.City{}, err
	}

	return city, nil
}
