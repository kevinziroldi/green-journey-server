package model

type Review struct {
	ReviewID             int    `gorm:"column:id_review;primaryKey;autoIncrement" json:"review_id"`
	CityID               int    `gorm:"column:id_city;type:integer;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"city_id"`
	UserID               int    `gorm:"column:id_user;type:integer;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user_id"`
	ReviewText           string `gorm:"column:review_text;type:text;not null" json:"review_text"`
	LocalTransportRating int    `gorm:"column:local_transport_rating;type:integer;not null" json:"local_transport_rating"`
	GreenSpacesRating    int    `gorm:"column:green_spaces_rating;type:integer;not null" json:"green_spaces_rating"`
	WasteBinsRating      int    `gorm:"column:waste_bins_rating;type:integer;not null" json:"waste_bins_rating"`
}

func (Review) TableName() string {
	return "review"
}
