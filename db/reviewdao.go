package db

import (
	"errors"
	"gorm.io/gorm"
	"green-journey-server/model"
)

const bestReviewsNumber = 5

type ReviewDAO struct {
	db *gorm.DB
}

func NewReviewDAO(db *gorm.DB) *ReviewDAO {
	return &ReviewDAO{db: db}
}

func (reviewDAO *ReviewDAO) GetReviewsById(reviewID int) (model.Review, error) {
	var review model.Review

	// get review
	result := reviewDAO.db.First(&review, reviewID)
	if result.Error != nil {
		return model.Review{}, result.Error
	}

	// inject data
	err := injectReviewData(&review)
	if err != nil {
		return model.Review{}, err
	}

	return review, nil
}

func (reviewDAO *ReviewDAO) GetReviewsByUser(userID int) ([]model.Review, error) {
	var reviews []model.Review

	// get reviews
	result := reviewDAO.db.Where("user_id = ?", userID).Find(&reviews)
	if result.Error != nil {
		return nil, result.Error
	}

	// inject data
	for i, _ := range reviews {
		err := injectReviewData(&reviews[i])
		if err != nil {
			return nil, err
		}
	}

	return reviews, nil
}

func (reviewDAO *ReviewDAO) GetReviewsByCity(cityID int) ([]model.Review, error) {
	var reviews []model.Review

	// get review
	result := reviewDAO.db.Where("id_city = ?", cityID).Find(&reviews)
	if result.Error != nil {
		return nil, result.Error
	}

	// inject data
	for i, _ := range reviews {
		err := injectReviewData(&reviews[i])
		if err != nil {
			return nil, err
		}
	}

	return reviews, nil
}

func (reviewDAO *ReviewDAO) GetCityReviewElementByCityID(cityID int) (model.CityReviewElement, error) {
	// get reviews
	reviews, err := reviewDAO.GetReviewsByCity(cityID)
	if err != nil {
		return model.CityReviewElement{}, err
	}

	// compute averages
	reviewsCount := len(reviews)
	sumLocalTransportRating := 0
	sumGreenSpacesRating := 0
	sumWasteBinsRating := 0
	for _, review := range reviews {
		sumLocalTransportRating += review.LocalTransportRating
		sumGreenSpacesRating += review.GreenSpacesRating
		sumWasteBinsRating += review.WasteBinsRating
	}
	averageLocalTransportRating := 0.0
	averageGreenSpacesRating := 0.0
	averageWasteBinsRating := 0.0
	if reviewsCount > 0 {
		averageLocalTransportRating = float64(sumLocalTransportRating) / float64(reviewsCount)
		averageGreenSpacesRating = float64(sumGreenSpacesRating) / float64(reviewsCount)
		averageWasteBinsRating = float64(sumWasteBinsRating) / float64(reviewsCount)
	}

	cityReviewElement := model.CityReviewElement{
		Reviews:                     reviews,
		AverageLocalTransportRating: averageLocalTransportRating,
		AverageGreenSpacesRating:    averageGreenSpacesRating,
		AverageWasteBinsRating:      averageWasteBinsRating,
	}

	return cityReviewElement, nil
}

func injectReviewData(review *model.Review) error {
	if review == nil {
		return errors.New("review is nil")
	}

	// get city
	cityDAO := NewCityDAO(GetDB())
	city, err := cityDAO.GetCityById(review.CityID)
	if err != nil {
		return err
	}

	// get user
	userDAO := NewUserDAO(GetDB())
	user, err := userDAO.GetUserById(review.UserID)
	if err != nil {
		return err
	}

	// inject data
	review.CityIata = *city.CityIata
	review.CountryCode = *city.CountryCode
	review.FirstName = user.FirstName
	review.LastName = user.LastName
	review.ScoreShortDistance = user.ScoreShortDistance
	review.ScoreLongDistance = user.ScoreLongDistance
	review.Badges = user.Badges // already injected by GetUserById

	return nil
}

func (reviewDAO *ReviewDAO) CreateReview(review *model.Review) error {
	// takes a pointer, in order to update the param struct

	// create transaction
	transaction := reviewDAO.db.Begin()
	if transaction.Error != nil {
		return transaction.Error
	}
	// rollback handled manually because I don't always want to rollback

	// save review
	result := transaction.Create(&review)
	if result.Error != nil {
		transaction.Rollback()
		return result.Error
	}

	// get reviews aggregated
	var reviewsAggregated model.ReviewsAggregated
	result = transaction.First(&reviewsAggregated, review.CityID)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			transaction.Rollback()
			return result.Error
		} else {
			// create the tuple
			reviewsAggregated = model.ReviewsAggregated{
				CityID:                  review.CityID,
				SumLocalTransportRating: review.LocalTransportRating,
				SumGreenSpacesRating:    review.GreenSpacesRating,
				SumWasteBinsRating:      review.WasteBinsRating,
				NumberRatings:           1,
			}
			result = transaction.Save(&reviewsAggregated)
			if result.Error != nil {
				transaction.Rollback()
				return result.Error
			}
		}
	} else {
		// tuple already existing, update
		reviewsAggregated.NumberRatings += 1
		reviewsAggregated.SumLocalTransportRating += review.LocalTransportRating
		reviewsAggregated.SumGreenSpacesRating += review.GreenSpacesRating
		reviewsAggregated.SumWasteBinsRating += review.WasteBinsRating
		result = transaction.Save(&reviewsAggregated)
		if result.Error != nil {
			transaction.Rollback()
			return result.Error
		}
	}

	// commit
	result = transaction.Commit()
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (reviewDAO *ReviewDAO) UpdateReview(review model.Review) error {
	// create transaction
	transaction := reviewDAO.db.Begin()
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

	// get reviews aggregated
	var reviewsAggregated model.ReviewsAggregated
	result := transaction.First(&reviewsAggregated, review.CityID)
	if result.Error != nil {
		// a tuple must be present
		return result.Error
	}

	// get old review
	var oldReview model.Review
	result = transaction.First(&oldReview, review.ReviewID)
	if result.Error != nil {
		return result.Error
	}

	// subtract old values
	reviewsAggregated.SumLocalTransportRating -= oldReview.LocalTransportRating
	reviewsAggregated.SumGreenSpacesRating -= oldReview.GreenSpacesRating
	reviewsAggregated.SumWasteBinsRating -= oldReview.WasteBinsRating

	// update review
	result = reviewDAO.db.Save(&review)
	if result.Error != nil {
		return result.Error
	}

	// sum new values
	reviewsAggregated.SumLocalTransportRating += review.LocalTransportRating
	reviewsAggregated.SumGreenSpacesRating += review.GreenSpacesRating
	reviewsAggregated.SumWasteBinsRating += review.WasteBinsRating

	// update reviewsAggregated
	result = transaction.Save(&reviewsAggregated)
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

func (reviewDAO *ReviewDAO) DeleteReview(reviewID int) error {
	// create transaction
	transaction := reviewDAO.db.Begin()
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

	// get review
	var review model.Review
	result := transaction.First(&review, reviewID)
	if result.Error != nil {
		return result.Error
	}

	// get reviews aggregated
	var reviewsAggregated model.ReviewsAggregated
	result = transaction.First(&reviewsAggregated, review.CityID)
	if result.Error != nil {
		// a tuple must be present
		return result.Error
	}

	// delete review
	result = transaction.Delete(&model.Review{}, reviewID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		transaction.Rollback()
		return errors.New("review not found")
	}

	// update reviewsAggregated
	if reviewsAggregated.NumberRatings > 1 {
		// there are other reviews

		reviewsAggregated.NumberRatings -= 1
		reviewsAggregated.SumLocalTransportRating -= review.LocalTransportRating
		reviewsAggregated.SumGreenSpacesRating -= review.GreenSpacesRating
		reviewsAggregated.SumWasteBinsRating -= review.WasteBinsRating

		result = transaction.Save(&reviewsAggregated)
		if result.Error != nil {
			return result.Error
		}
	} else {
		// no other review present

		// delete reviews aggregated
		result = transaction.Delete(&model.ReviewsAggregated{}, reviewsAggregated.CityID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			transaction.Rollback()
			return errors.New("reviews aggregated entry not found")
		}
	}

	// commit
	result = transaction.Commit()
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (reviewDAO *ReviewDAO) GetBestReviews() ([]model.CityReviewElement, error) {
	var reviewsAggregatedList []model.ReviewsAggregated

	err := reviewDAO.db.
		Table("reviews_aggregated").
		Select("*, ((sum_local_transport_rating / NULLIF(number_ratings, 0)) + (sum_green_spaces_rating / NULLIF(number_ratings, 0)) + (sum_waste_bins_rating / NULLIF(number_ratings, 0))) AS total_average").
		Order("total_average DESC").
		Limit(5).
		Scan(&reviewsAggregatedList)

	if err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return []model.CityReviewElement{}, nil
		} else {
			return nil, err.Error
		}
	}

	// inject average
	for i, _ := range reviewsAggregatedList {
		// inject averages
		reviewsAggregatedList[i].AverageLocalTransportRating = float64(reviewsAggregatedList[i].SumLocalTransportRating) / float64(reviewsAggregatedList[i].NumberRatings)
		reviewsAggregatedList[i].AverageGreenSpacesRating = float64(reviewsAggregatedList[i].SumGreenSpacesRating) / float64(reviewsAggregatedList[i].NumberRatings)
		reviewsAggregatedList[i].AverageWasteBinsRating = float64(reviewsAggregatedList[i].SumWasteBinsRating) / float64(reviewsAggregatedList[i].NumberRatings)
	}

	var bestReviewsElements []model.CityReviewElement
	for i := 0; i < len(reviewsAggregatedList) && i < bestReviewsNumber; i++ {
		// get reviews
		reviews, err1 := reviewDAO.GetReviewsByCity(reviewsAggregatedList[i].CityID)
		if err1 != nil {
			return nil, err1
		}

		// build BestReviewsElement
		bestReviewsElement := model.CityReviewElement{
			Reviews:                     reviews,
			AverageLocalTransportRating: reviewsAggregatedList[i].AverageLocalTransportRating,
			AverageGreenSpacesRating:    reviewsAggregatedList[i].AverageGreenSpacesRating,
			AverageWasteBinsRating:      reviewsAggregatedList[i].AverageWasteBinsRating,
		}

		bestReviewsElements = append(bestReviewsElements, bestReviewsElement)
	}

	return bestReviewsElements, nil
}
