package db

import (
	"errors"
	"gorm.io/gorm"
	"green-journey-server/model"
	"sort"
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
	result := reviewDAO.db.Where("city_id = ?", cityID).Find(&reviews)
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

func (reviewDAO *ReviewDAO) CreateReview(review model.Review) error {
	// create transaction
	transaction := db.Begin()
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
				transaction.Rollback()
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
	reviewsAggregated.CountLocalTransportRating -= 1
	reviewsAggregated.CountGreenSpacesRating -= 1
	reviewsAggregated.CountWasteBinsRating -= 1
	reviewsAggregated.SumLocalTransportRating -= review.LocalTransportRating
	reviewsAggregated.SumGreenSpacesRating -= review.GreenSpacesRating
	reviewsAggregated.SumWasteBinsRating -= review.WasteBinsRating

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

func (reviewDAO *ReviewDAO) GetBestReviews() ([]model.BestReviewElement, error) {
	// get all cities
	cityDAO := NewCityDAO(GetDB())
	cities, err := cityDAO.GetCities()
	if err != nil {
		return nil, err
	}

	// for every city, get aggregate data
	var reviewsAggregatedList []model.ReviewsAggregated
	for _, city := range cities {
		var reviewsAggregated model.ReviewsAggregated
		result := reviewDAO.db.First(&reviewsAggregated, city.CityID)
		if result.Error != nil {
			// city doesn't contain tuples or other error
			// just skip one city
			continue
		}
		// inject averages
		reviewsAggregated.AverageLocalTransportRating = float64(reviewsAggregated.SumLocalTransportRating) / float64(reviewsAggregated.CountLocalTransportRating)
		reviewsAggregated.AverageGreenSpacesRating = float64(reviewsAggregated.SumGreenSpacesRating) / float64(reviewsAggregated.CountGreenSpacesRating)
		reviewsAggregated.AverageWasteBinsRating = float64(reviewsAggregated.SumWasteBinsRating) / float64(reviewsAggregated.CountWasteBinsRating)
		// append
		reviewsAggregatedList = append(reviewsAggregatedList, reviewsAggregated)
	}

	// sort slice according to higher sum of averages, if tie according to number of reviews
	sort.Slice(reviewsAggregatedList, func(i, j int) bool {
		sumAveragesI := reviewsAggregatedList[i].AverageLocalTransportRating + reviewsAggregatedList[i].AverageGreenSpacesRating + reviewsAggregatedList[i].AverageWasteBinsRating
		sumAveragesJ := reviewsAggregatedList[j].AverageLocalTransportRating + reviewsAggregatedList[j].AverageGreenSpacesRating + reviewsAggregatedList[j].AverageWasteBinsRating
		countI := reviewsAggregatedList[i].CountLocalTransportRating + reviewsAggregatedList[i].CountGreenSpacesRating + reviewsAggregatedList[i].CountWasteBinsRating
		countJ := reviewsAggregatedList[j].CountLocalTransportRating + reviewsAggregatedList[j].CountGreenSpacesRating + reviewsAggregatedList[j].CountWasteBinsRating

		if sumAveragesI == sumAveragesJ {
			return countI >= countJ
		}
		// else
		return sumAveragesI > sumAveragesJ
	})

	var bestReviewsElements []model.BestReviewElement
	for i := 0; i < len(reviewsAggregatedList) && i < bestReviewsNumber; i++ {
		// get reviews
		reviews, err1 := reviewDAO.GetReviewsByCity(reviewsAggregatedList[i].CityID)
		if err1 != nil {
			return nil, err1
		}

		// build BestReviewsElement
		bestReviewsElement := model.BestReviewElement{
			Reviews:                     reviews,
			CountLocalTransportRating:   reviewsAggregatedList[i].CountLocalTransportRating,
			CountGreenSpacesRating:      reviewsAggregatedList[i].CountGreenSpacesRating,
			CountWasteBinsRating:        reviewsAggregatedList[i].CountWasteBinsRating,
			AverageLocalTransportRating: reviewsAggregatedList[i].AverageLocalTransportRating,
			AverageGreenSpacesRating:    reviewsAggregatedList[i].AverageGreenSpacesRating,
			AverageWasteBinsRating:      reviewsAggregatedList[i].AverageWasteBinsRating,
		}

		bestReviewsElements = append(bestReviewsElements, bestReviewsElement)
	}

	return bestReviewsElements, nil
}
