package model

type City struct {
	CityID      int     `gorm:"column:id_city;primaryKey;autoIncrement" json:"city_id"`
	CityIata    *string `gorm:"column:city_iata;type:text" json:"city_iata"`
	CityName    string  `gorm:"column:city_name;type:text;not null" json:"city_name"`
	CountryName *string `gorm:"column:country_name;type:text" json:"country_name"`
	CountryCode *string `gorm:"column:country_code;type:text" json:"country_code"`
	Continent   *string `gorm:"column:continent;type:text" json:"continent"`
}

func (City) TableName() string {
	return "city"
}
