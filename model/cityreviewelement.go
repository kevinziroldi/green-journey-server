package model

// CityReviewElement is the struct that will be sent to the client to display reviews
type CityReviewElement struct {
	Reviews                     []Review `json:"reviews"`
	CountLocalTransportRating   int      `json:"count_local_transport_rating"`
	CountGreenSpacesRating      int      `json:"count_green_spaces_rating"`
	CountWasteBinsRating        int      `json:"count_waste_bins_rating"`
	AverageLocalTransportRating float64  `json:"average_local_transport_rating"`
	AverageGreenSpacesRating    float64  `json:"average_green_spaces_rating"`
	AverageWasteBinsRating      float64  `json:"average_waste_bins_rating"`
}
