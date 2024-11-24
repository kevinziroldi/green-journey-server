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

func (travelDAO *TravelDAO) CreateTravel(travelDetails model.TravelDetails) (model.TravelDetails, error) {
	// create transaction
	transaction := db.Begin()
	if transaction.Error != nil {
		return model.TravelDetails{}, transaction.Error
	}

	defer func() {
		if r := recover(); r != nil {
			transaction.Rollback()
			panic(r)
		} else if transaction.Error != nil {
			transaction.Rollback()
		}
	}()

	// create travel entry
	result := transaction.Create(&travelDetails.Travel)
	if result.Error != nil {
		return model.TravelDetails{}, result.Error
	}

	// create segment entries
	for i, _ := range travelDetails.Segments {
		// set travelID to all segments
		travelDetails.Segments[i].TravelID = travelDetails.Travel.TravelID
		travelDetails.Segments[i].Hour = travelDetails.Segments[i].Date
		result = transaction.Create(&travelDetails.Segments[i])
		if result.Error != nil {
			return model.TravelDetails{}, result.Error
		}
	}

	result = transaction.Commit()
	if result.Error != nil {
		return model.TravelDetails{}, result.Error
	}

	return travelDetails, nil
}

func (travelDAO *TravelDAO) GetTravelRequestsByUserId(userID int) ([]model.TravelDetails, error) {
	var travels []model.Travel
	var travelDetailsList []model.TravelDetails

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

func (travelDAO *TravelDAO) GetTravelDetailsByTravelID(travelID int) (model.TravelDetails, error) {
	// get travel
	travel, err := travelDAO.GetTravelById(travelID)
	if err != nil {
		return model.TravelDetails{}, err
	}

	// get segments
	var segments []model.Segment
	result := db.Where("id_travel = ?", travel.TravelID).Find(&segments)
	if result.Error != nil {
		return model.TravelDetails{}, result.Error
	}

	return model.TravelDetails{Travel: travel, Segments: segments}, nil
}

func (travelDAO *TravelDAO) UpdateTravel(travel model.Travel, deltaScore float64) error {
	// create transaction
	transaction := db.Begin()
	if transaction.Error != nil {
		return transaction.Error
	}

	defer func() {
		if r := recover(); r != nil {
			transaction.Rollback()
			panic(r)
		} else if transaction.Error != nil {
			transaction.Rollback()
		}
	}()

	// save updated travel
	result := transaction.Save(&travel)

	// update user score
	userDAO := NewUserDAO(GetDB())
	user, err := userDAO.GetUserById(travel.UserID)
	if err != nil {
		// out of transaction, so rollback
		transaction.Rollback()
		return err
	}
	if deltaScore < 0.0 {
		return err
	}
	user.Score += deltaScore
	result = transaction.Save(&user)
	if result.Error != nil {
		return result.Error
	}

	// commit
	result = transaction.Commit()
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (travelDAO *TravelDAO) DeleteTravel(travelID int, deltaScore float64) error {
	// create transaction
	transaction := db.Begin()
	if transaction.Error != nil {
		return transaction.Error
	}

	defer func() {
		if r := recover(); r != nil {
			transaction.Rollback()
			panic(r)
		} else if transaction.Error != nil {
			transaction.Rollback()
		}
	}()

	// get user id
	var travel model.Travel
	err := transaction.First(&travel, travelID)
	if err != nil {
		return err.Error
	}
	userID := travel.UserID

	// delete travel
	result := transaction.Delete(&model.Travel{}, travelID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// manually rollback
		transaction.Rollback()
		return errors.New("travel not found")
	}

	// update user score
	userDAO := NewUserDAO(GetDB())
	user, err1 := userDAO.GetUserById(userID)
	if err1 != nil {
		transaction.Rollback()
		return err1
	}

	user.Score -= deltaScore
	result = transaction.Save(&user)
	if result.Error != nil {
		return result.Error
	}

	// commit
	result = transaction.Commit()
	if result.Error != nil {
		return result.Error
	}

	return nil
}
