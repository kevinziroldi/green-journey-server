package model

// CityReviewElement is the struct that will be sent to the client to display reviews
type CityReviewElement struct {
	Reviews                     []Review `json:"reviews"`
	AverageLocalTransportRating float64  `json:"average_local_transport_rating"`
	AverageGreenSpacesRating    float64  `json:"average_green_spaces_rating"`
	AverageWasteBinsRating      float64  `json:"average_waste_bins_rating"`
	HasPrevious                 bool     `json:"has_previous"`
	HasNext                     bool     `json:"has_next"`
	NumReviews                  int      `json:"num_reviews"`
}
