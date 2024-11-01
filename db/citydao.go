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

func (cityDAO *CityDAO) GetCityById(cityID int) (model.City, error) {
	var city model.City
	result := cityDAO.db.First(&city, cityID)
	return city, result.Error
}

func (cityDAO *CityDAO) GetCityByName(name string) (model.City, error) {
	var city model.City

	result := cityDAO.db.Where("city_name = ?", name).First(&city)
	if result.Error != nil {
		return model.City{}, result.Error
	}

	return city, nil
}

func (cityDAO *CityDAO) GetCityByCoordinates(latitude, longitude, delta float64) (model.City, error) {
	var city model.City

	minLatitude := latitude - delta
	maxLatitude := latitude + delta
	minLongitude := longitude - delta
	maxLongitude := longitude + delta

	query := fmt.Sprintf(
		"(latitude - %f)*(latitude - %f) + (longitude - %f)*(longitude - %f)",
		latitude, latitude, longitude, longitude,
	)

	result := cityDAO.db.Where("latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ?", minLatitude, maxLatitude, minLongitude, maxLongitude).Order(query).First(&city)

	if result.Error != nil {
		return model.City{}, result.Error
	}

	return city, nil
}

func (cityDAO *CityDAO) GetCityByCityIata(cityIata string) (model.City, error) {
	var city model.City

	result := cityDAO.db.Where("city_iata = ?", cityIata).First(&city)
	if result.Error != nil {
		return model.City{}, result.Error
	}

	return city, nil
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

func (cityDAO *CityDAO) GetAirportByIata(iata string) (model.Airport, error) {
	var airport model.Airport

	result := cityDAO.db.Where("airport_iata = ?", iata).First(&airport)
	if result.Error != nil {
		return model.Airport{}, result.Error
	}

	return airport, nil
}
