package model

type City struct {
	CityID      int     `gorm:"column:id_city;primaryKey;autoIncrement" json:"city_id"`
	CityName    string  `gorm:"column:city_name;type:text;not null" json:"city_name"`
	CountryName *string `gorm:"column:country_name;type:text" json:"country_name"` // can be nil, pointer
	CountryCode *string `gorm:"column:country_code;type:text" json:"country_code"` // can be nil, pointer
	Iata        *string `gorm:"column:iata;type:text" json:"iata"`                 // can be nil, pointer
	Latitude    float64 `gorm:"column:latitude;type:numeric;not null" json:"latitude"`
	Longitude   float64 `gorm:"column:longitude;type:numeric;not null" json:"longitude"`
}

func (City) TableName() string {
	return "city"
}
