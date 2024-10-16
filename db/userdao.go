package db

import (
	"errors"
	"gorm.io/gorm"
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
	return user, result.Error
}

func (userDAO *UserDAO) GetUserByFirebaseUID(firebaseUID int) (model.User, error) {
	var user model.User
	result := userDAO.db.Where("firebase_uid = ?", firebaseUID).First(&user)
	return user, result.Error
}

func (userDAO *UserDAO) AddUser(user model.User) error {
	result := db.Create(&user)
	return result.Error
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
