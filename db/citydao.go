package db

import (
	"fmt"
	"gorm.io/gorm"
	"green-journey-server/model"
)

type CityDAO struct {
	db *gorm.DB
}

func NewCityDAO(db *gorm.DB) *CityDAO {
	return &CityDAO{db: db}
}

func (cityDAO *CityDAO) CreateCity(city *model.City) error {
	// takes a pointer, in order to update the param struct
	result := db.Create(city)
	return result.Error
}

func (cityDAO *CityDAO) CreateAirport(airport *model.Airport) error {
	// takes a pointer, in order to update the param struct
	result := db.Create(airport)
	return result.Error
}

func (cityDAO *CityDAO) GetCities() ([]model.City, error) {
	var cities []model.City
	result := db.Find(&cities)
	return cities, result.Error
}

func (cityDAO *CityDAO) GetCityById(cityID int) (model.City, error) {
	var city model.City
	result := cityDAO.db.First(&city, cityID)
	return city, result.Error
}

// GetCityByIataAndCountryCode the city iata and the country code can identify the city uniquely
func (cityDAO *CityDAO) GetCityByIataAndCountryCode(cityIata, countryCode string) (model.City, error) {
	var city model.City

	result := cityDAO.db.Where("city_iata = ? AND country_code = ?", cityIata, countryCode).First(&city)
	if result.Error != nil {
		return model.City{}, result.Error
	}

	return city, nil
}

// GetCityByNameAndCountry used for second class cities, i.e. cities that represent an intermedia stop
// for a transit travel
func (cityDAO *CityDAO) GetCityByNameAndCountry(cityName, countryName string) (model.City, error) {
	var city model.City

	// get city by city_name and country_name
	result := cityDAO.db.Where("city_name = ? AND country_name = ?", cityName, countryName).First(&city)
	if result.Error != nil {
		return model.City{}, result.Error
	}

	return city, nil
}

func (cityDAO *CityDAO) GetAirportByAirportIata(airportIata string) (model.Airport, error) {
	var airport model.Airport
	result := cityDAO.db.Where("airport_iata = ?", airportIata).First(&airport)
	return airport, result.Error
}

func (cityDAO *CityDAO) GetCityByAirportIata(airportIata string) (model.City, error) {
	var city model.City

	err := cityDAO.db.Joins("JOIN airport ON airport.id_city = city.id_city").
		Where("airport.airport_iata = ?", airportIata).
		First(&city).Error

	if err != nil {
		return model.City{}, err
	}

	return city, nil
}

func (cityDAO *CityDAO) UpdateCityById(cityID int, fields map[string]interface{}) (model.City, error) {
	result := cityDAO.db.Model(&model.City{}).Where("id_city = ?", cityID).Updates(fields)

	if result.Error != nil {
		return model.City{}, result.Error
	}
	if result.RowsAffected == 0 {
		return model.City{}, fmt.Errorf("no city found with id %d", cityID)
	}

	var city model.City
	err := cityDAO.db.First(&city, cityID).Error
	if err != nil {
		return model.City{}, err
	}

	return city, nil
}
