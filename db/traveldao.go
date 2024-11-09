package db

import (
	"errors"
	"gorm.io/gorm"
	"green-journey-server/model"
)

type TravelDAO struct {
	db *gorm.DB
}

func NewTravelDAO(db *gorm.DB) *TravelDAO {
	return &TravelDAO{db: db}
}

func (travelDAO *TravelDAO) CreateTravel(travelDetails model.TravelDetails) error {
	// create transaction
	transaction := db.Begin()
	if transaction.Error != nil {
		return transaction.Error
	}

	// create travel entry
	result := transaction.Create(&travelDetails.Travel)
	if result.Error != nil {
		transaction.Rollback()
		return result.Error
	}

	// create segment entries
	for _, segment := range travelDetails.Segments {
		// set travelID to all segments
		segment.TravelID = travelDetails.Travel.TravelID
		result = transaction.Create(&segment)
		if result.Error != nil {
			transaction.Rollback()
			return result.Error
		}
	}

	result = transaction.Commit()
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (travelDAO *TravelDAO) GetTravelRequestsByUserId(userID int) ([]model.TravelDetails, error) {
	var travels []model.Travel
	var travelDetailsList []model.TravelDetails
	cityDAO := NewCityDAO(GetDB())

	// get travels
	result := db.Where("id_user = ?", userID).Find(&travels)
	if result.Error != nil {
		return nil, result.Error
	}

	// get segments for every travel
	for _, travel := range travels {
		var segments []model.Segment

		// get segments
		result = db.Where("id_travel = ?", travel.TravelID).Find(&segments)
		if result.Error != nil {
			return nil, result.Error
		}

		// add departure and destination to segments
		for i, _ := range segments {
			originCity, err := cityDAO.GetCityById(segments[i].DepartureId)
			if err != nil {
				return nil, err
			}
			destinationCity, err := cityDAO.GetCityById(segments[i].DestinationId)
			if err != nil {
				return nil, err
			}

			segments[i].Departure = originCity.CityName
			segments[i].Destination = destinationCity.CityName
		}

		// add to travelRequests
		travelDetailsList = append(travelDetailsList, model.TravelDetails{
			Travel: travel, Segments: segments,
		})
	}

	return travelDetailsList, nil
}

func (travelDAO *TravelDAO) GetTravelById(travelID int) (model.Travel, error) {
	var travel model.Travel
	result := travelDAO.db.First(&travel, travelID)
	return travel, result.Error
}

func (travelDAO *TravelDAO) UpdateTravel(travel model.Travel) error {
	result := db.Save(&travel)
	return result.Error
}

func (travelDAO *TravelDAO) DeleteTravel(id int) error {
	result := db.Delete(&model.Travel{}, id)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("travel not found")
	}

	return nil
}
