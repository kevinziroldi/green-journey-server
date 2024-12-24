package model

// ReviewsAggregated is a struct corresponding to a DB table, that contains
// aggregate data about reviews for a city: it is used to avoid computing all values
// when a user asks for best reviewed cities
type ReviewsAggregated struct {
	CityID                      int     `gorm:"column:id_city;primaryKey;constraint:OnUpdate:NO ACTION,OnDelete:NO ACTION"`
	SumLocalTransportRating     int     `gorm:"column:sum_local_transport_rating;type:integer;not null"`
	SumGreenSpacesRating        int     `gorm:"column:sum_green_spaces_rating;type:integer;not null"`
	SumWasteBinsRating          int     `gorm:"column:sum_waste_bins_rating;type:integer;not null"`
	NumberRatings               int     ` gorm:"column:number_ratings;type:integer;not null"`
	AverageLocalTransportRating float64 `gorm:"-"`
	AverageGreenSpacesRating    float64 `gorm:"-"`
	AverageWasteBinsRating      float64 `gorm:"-"`
}

func (ReviewsAggregated) TableName() string {
	return "reviews_aggregated"
}
