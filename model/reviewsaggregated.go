package model

type ReviewsAggregated struct {
	CityID                    int `gorm:"column:id_city;primaryKey;autoIncrement;constraint:OnUpdate:NO ACTION,OnDelete:NO ACTION"`
	SumLocalTransportRating   int `gorm:"column:sum_local_transport_rating;type:integer;not null"`
	SumGreenSpacesRating      int `gorm:"column:sum_green_spaces_rating;type:integer;not null"`
	SumWasteBinsRating        int `gorm:"column:sum_waste_bins_rating;type:integer;not null"`
	CountLocalTransportRating int `gorm:"column:count_local_transport_rating;type:integer;not null"`
	CountGreenSpacesRating    int `gorm:"column:count_green_spaces_rating;type:integer;not null"`
	CountWasteBinsRating      int `gorm:"column:count_waste_bins_rating;type:integer;not null"`
}

func (ReviewsAggregated) TableName() string {
	return "reviews_aggregated"
}
