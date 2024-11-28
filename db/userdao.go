package db

import (
	"errors"
	"gorm.io/gorm"
	"green-journey-server/model"
)

type UserDAO struct {
	db *gorm.DB
}

const travelCoefficient = 10.0
const compensationCoefficient = 10.0

const distanceLowLimit = 3000
const distanceMidLimit = 3000
const distanceHighLimit = 3000

const ecologicalChoiceLowLimit = 1  // TODO
const ecologicalChoiceMidLimit = 2  // TODO
const ecologicalChoiceHighLimit = 3 // TODO

const compensationLowLimit = 1  // TODO
const compensationMidLimit = 2  // TODO
const compensationHighLimit = 3 // TODO

const numTravelsLowLimit = 5
const numTravelsMidLimit = 10
const numTravelsHighLimit = 30

func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{db: db}
}

func (userDAO *UserDAO) GetUserById(id int) (model.User, error) {
	var user model.User
	result := userDAO.db.First(&user, id)

	// inject badges, not present in db
	err := userDAO.InjectBadges(&user)
	if err != nil {
		return model.User{}, err
	}

	return user, result.Error
}

func (userDAO *UserDAO) GetUserByFirebaseUID(firebaseUID string) (model.User, error) {
	var user model.User
	result := userDAO.db.Where("firebase_uid = ?", firebaseUID).First(&user)

	// inject badges, not present in db
	err := userDAO.InjectBadges(&user)
	if err != nil {
		return model.User{}, err
	}

	return user, result.Error
}

func (userDAO *UserDAO) InjectBadges(user *model.User) error {
	badges := []model.Badge{}

	// compute data
	totalDistance := 0.0
	totalCO2Emitted := 0.0
	totalCO2Compensated := 0.0
	numTravels := 0
	travelDAO := NewTravelDAO(GetDB())
	travels, err := travelDAO.GetTravelRequestsByUserId(user.UserID)
	if err != nil {
		return err
	}
	for _, travelDetails := range travels {
		numTravels++
		totalCO2Compensated += travelDetails.Travel.CO2Compensated

		for _, segment := range travelDetails.Segments {
			totalDistance += segment.Distance
			totalCO2Emitted += segment.CO2Emitted
		}
	}

	// compute badges
	if totalDistance >= distanceLowLimit {
		badges = append(badges, model.BadgeDistanceLow)
	}
	if totalDistance >= distanceMidLimit {
		badges = append(badges, model.BadgeDistanceMid)
	}
	if totalDistance >= distanceHighLimit {
		badges = append(badges, model.BadgeDistanceHigh)
	}
	ecologicalChoiceValue := travelCoefficient * totalDistance / (0.001 + totalCO2Emitted)
	if ecologicalChoiceValue >= ecologicalChoiceLowLimit {
		badges = append(badges, model.BadgeEcologicalChoiceLow)
	}
	if ecologicalChoiceValue >= ecologicalChoiceMidLimit {
		badges = append(badges, model.BadgeEcologicalChoiceMid)
	}
	if ecologicalChoiceValue >= ecologicalChoiceHighLimit {
		badges = append(badges, model.BadgeEcologicalChoiceHigh)
	}
	compensationValue := compensationCoefficient * totalCO2Compensated / (0.001 + totalCO2Emitted)
	if compensationValue >= compensationLowLimit {
		badges = append(badges, model.BadgeCompensationLow)
	}
	if compensationValue >= compensationMidLimit {
		badges = append(badges, model.BadgeCompensationMid)
	}
	if compensationValue >= compensationHighLimit {
		badges = append(badges, model.BadgeCompensationHigh)
	}
	if numTravels >= numTravelsLowLimit {
		badges = append(badges, model.BadgeTravelsNumberLow)
	}
	if numTravels >= numTravelsMidLimit {
		badges = append(badges, model.BadgeTravelsNumberMid)
	}
	if numTravels >= numTravelsHighLimit {
		badges = append(badges, model.BadgeTravelsNumberHigh)
	}

	// inject badges
	user.Badges = badges

	return nil
}

func (userDAO *UserDAO) AddUser(user model.User) (model.User, error) {
	result := db.Create(&user)
	return user, result.Error
}

func (userDAO *UserDAO) UpdateUser(user model.User) error {
	result := db.Save(&user)
	return result.Error
}

func (userDAO *UserDAO) DeleteUser(id int) error {
	result := db.Delete(&model.User{}, id)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}
