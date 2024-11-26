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

func (reviewDAO *ReviewDAO) GetReviewsByUser(userID int) ([]model.Review, error) {
	var reviews []model.Review
	result := reviewDAO.db.Where("user_id = ?", userID).Find(&reviews)
	return reviews, result.Error
}

func (reviewDAO *ReviewDAO) GetReviewsByCity(cityID int) ([]model.Review, error) {
	var reviews []model.Review
	result := reviewDAO.db.Where("city_id = ?", cityID).Find(&reviews)
	return reviews, result.Error
}

func (reviewDAO *ReviewDAO) CreateReview(review model.Review) error {
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

	// save review
	result := transaction.Create(&review)
	if result.Error != nil {
		return result.Error
	}

	// get reviews aggregated
	var reviewsAggregated model.ReviewsAggregated
	result = transaction.First(&reviewsAggregated, review.CityID)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		} else {
			// create the tuple
			reviewsAggregated = model.ReviewsAggregated{
				CityID:                    review.CityID,
				SumLocalTransportRating:   review.LocalTransportRating,
				SumGreenSpacesRating:      review.GreenSpacesRating,
				SumWasteBinsRating:        review.WasteBinsRating,
				CountLocalTransportRating: 1,
				CountGreenSpacesRating:    1,
				CountWasteBinsRating:      1,
			}
			result = transaction.Save(&reviewsAggregated)
			if result.Error != nil {
				return result.Error
			}
		}
	} else {
		// tuple already existing, update
		reviewsAggregated.CountLocalTransportRating += 1
		reviewsAggregated.CountGreenSpacesRating += 1
		reviewsAggregated.CountWasteBinsRating += 1
		reviewsAggregated.SumLocalTransportRating += review.LocalTransportRating
		reviewsAggregated.SumGreenSpacesRating += review.GreenSpacesRating
		reviewsAggregated.SumWasteBinsRating += review.WasteBinsRating
		result = transaction.Save(&reviewsAggregated)
		if result.Error != nil {
			return result.Error
		}
	}

	return nil
}

func (reviewDAO *ReviewDAO) UpdateReview(review model.Review) error {
	result := reviewDAO.db.Save(&review)

	// TODO update reviewsaggregated in a transaction:
	//  count_... remains unchanged
	//  for every score, sum - old score + new score
	//  tuple in the table must be present

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

	// TODO update reviewsaggregated in a transaction:
	//  for every count_... -1
	//  for every sum_... - score of the review I am deleting
	//  tuple in the table must be present

	return nil
}
