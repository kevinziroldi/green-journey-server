package db

import (
	"errors"
	"gorm.io/gorm"
	"green-journey-server/model"
)

type ReviewDAO struct {
	db *gorm.DB
}

func NewReviewDAO(db *gorm.DB) *ReviewDAO {
	return &ReviewDAO{db: db}
}

func (reviewDAO *ReviewDAO) GetReviewsById(reviewID int) (model.Review, error) {
	var review model.Review
	result := reviewDAO.db.First(&review, reviewID)
	return review, result.Error
}

func (reviewDAO *ReviewDAO) CreateReview(review model.Review) error {
	result := reviewDAO.db.Create(&review)
	return result.Error
}

func (reviewDAO *ReviewDAO) UpdateReview(review model.Review) error {
	result := reviewDAO.db.Save(&review)
	return result.Error
}

func (reviewDAO *ReviewDAO) DeleteReview(reviewID int) error {
	result := reviewDAO.db.Delete(&model.Review{}, reviewID)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("review not found")
	}

	return nil
}

func (reviewDAO *ReviewDAO) GetReviewsByCity(cityID int) ([]model.Review, error) {
	var reviews []model.Review
	result := reviewDAO.db.Where("city_id = ?", cityID).Find(&reviews)
	return reviews, result.Error
}

func (reviewDAO *ReviewDAO) GetReviewsByUser(userID int) ([]model.Review, error) {
	var reviews []model.Review
	result := reviewDAO.db.Where("user_id = ?", userID).Find(&reviews)
	return reviews, result.Error
}
