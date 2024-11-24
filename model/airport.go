package model

type Airport struct {
	AirportID   int     `gorm:"column:id_airport;primaryKey;autoIncrement" json:"airport_id"`
	AirportName string  `gorm:"column:airport_name;type:text;not null" json:"airport_name"`
	AirportIata string  `gorm:"column:airport_iata;type:text;not null" json:"airport_iata"`
	Latitude    float64 `gorm:"column:latitude;type:numeric;not null" json:"latitude"`
	Longitude   float64 `gorm:"column:longitude;type:numeric;not null" json:"longitude"`
	CityID      int     `gorm:"column:id_city;type:integer;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"city_id"`
}

func (Airport) TableName() string {
	return "airport"
}
