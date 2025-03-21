package db

import (
	"errors"
	"gorm.io/gorm"
	"green-journey-server/internals"
	"green-journey-server/model"
)

type UserDAO struct {
	db *gorm.DB
}

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

func (userDAO *UserDAO) GetUserByIdNoBadges(id int) (model.User, error) {
	var user model.User
	result := userDAO.db.First(&user, id)

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
	// empty slice if no badge
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
		if travelDetails.Travel.Confirmed {
			numTravels++
			totalCO2Compensated += travelDetails.Travel.CO2Compensated

			for _, segment := range travelDetails.Segments {
				totalDistance += segment.Distance
				totalCO2Emitted += segment.CO2Emitted
			}
		}
	}

	// compute badges
	distanceBadge, err := internals.ComputeDistanceBadge(totalDistance)
	if err == nil {
		badges = append(badges, distanceBadge)
	}
	ecologicalChoiceBadge, err := internals.ComputeEcologicalChoiceBadge(totalDistance, totalCO2Emitted)
	if err == nil {
		badges = append(badges, ecologicalChoiceBadge)
	}
	compensationBadge, err := internals.ComputeCompensationBadge(totalCO2Compensated, totalCO2Emitted)
	if err == nil {
		badges = append(badges, compensationBadge)
	}
	numTravelsBadge, err := internals.ComputeTravelsNumberCoefficient(numTravels)
	if err == nil {
		badges = append(badges, numTravelsBadge)
	}

	// inject badges
	user.Badges = badges
	return nil
}

func (userDAO *UserDAO) AddUser(user model.User) (model.User, error) {
	result := userDAO.db.Create(&user)
	return user, result.Error
}

func (userDAO *UserDAO) UpdateUser(user model.User) error {
	result := userDAO.db.Save(&user)
	return result.Error
}

func (userDAO *UserDAO) DeleteUser(id int) error {
	result := userDAO.db.Delete(&model.User{}, id)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}
