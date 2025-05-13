package db

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"green-journey-server/model"
)

const bestReviewsNumber = 5
const reviewsPageSize = 10

type ReviewDAO struct {
	db *gorm.DB
}

func NewReviewDAO(db *gorm.DB) *ReviewDAO {
	return &ReviewDAO{db: db}
}

func (reviewDAO *ReviewDAO) GetReviewById(reviewID int) (model.Review, error) {
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

func (reviewDAO *ReviewDAO) GetReviewByUserIDAndCityID(userID int, cityID int) (*model.Review, error) {
	var review model.Review

	// get review
	result := reviewDAO.db.Where("id_user = ? AND id_city = ?", userID, cityID).First(&review)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}

	// inject review data
	err := injectReviewData(&review)
	if err != nil {
		return nil, err
	}

	return &review, nil
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

func (reviewDAO *ReviewDAO) GetNextReviews(cityID int, reviewID int) (model.CityReviewElement, error) {
	// get review
	review, err := reviewDAO.GetReviewById(reviewID)
	if err != nil {
		return model.CityReviewElement{}, err
	}
	if review.CityID != cityID {
		return model.CityReviewElement{}, fmt.Errorf("wrong city id and review id")
	}

	// get next reviews
	var reviews = []model.Review{}
	result := db.
		Where("(id_city = ?) AND ((date_time < ?) OR (date_time = ? AND id_review < ?))", cityID, review.DateTime, review.DateTime, review.ReviewID).
		Order("date_time desc, id_review desc").
		Limit(reviewsPageSize + 1).
		Find(&reviews)
	if result.Error != nil {
		return model.CityReviewElement{}, err
	}

	// inject data
	for i, _ := range reviews {
		err = injectReviewData(&reviews[i])
		if err != nil {
			return model.CityReviewElement{}, err
		}
	}

	// previous and next page
	hasNext := len(reviews) == reviewsPageSize+1
	if hasNext {
		// remove extra element
		reviews = reviews[:reviewsPageSize]
	}

	// compute averages
	if len(reviews) == 0 {
		return model.CityReviewElement{}, fmt.Errorf("no review found")
	}
	averageLocalTransportRating, averageGreenSpacesRating, averageWasteBinsRating, err := reviewDAO.computeReviewsAverages(reviews[0].CityID)
	if err != nil {
		return model.CityReviewElement{}, fmt.Errorf("error computing average")
	}

	// get number of reviews
	var numReviews int64
	result = db.Model(&model.Review{}).Where("id_city = ?", cityID).Count(&numReviews)
	if result.Error != nil {
		return model.CityReviewElement{}, result.Error
	}

	cityReviewElement := model.CityReviewElement{
		Reviews:                     reviews,
		AverageLocalTransportRating: averageLocalTransportRating,
		AverageGreenSpacesRating:    averageGreenSpacesRating,
		AverageWasteBinsRating:      averageWasteBinsRating,
		HasPrevious:                 true,
		HasNext:                     hasNext,
		NumReviews:                  int(numReviews),
	}

	return cityReviewElement, nil
}

func (reviewDAO *ReviewDAO) GetPreviousReviews(cityID int, reviewID int) (model.CityReviewElement, error) {
	// get review
	review, err := reviewDAO.GetReviewById(reviewID)
	if err != nil {
		return model.CityReviewElement{}, err
	}
	if review.CityID != cityID {
		return model.CityReviewElement{}, fmt.Errorf("wrong city id and review id")
	}

	// get previous reviews
	var reviews []model.Review

	result := db.
		Where("(id_city = ?) AND ((date_time > ?) OR (date_time = ? AND id_review > ?))", cityID, review.DateTime, review.DateTime, review.ReviewID).
		Order("date_time asc, id_review asc").
		Limit(reviewsPageSize + 1).
		Find(&reviews)
	if result.Error != nil {
		return model.CityReviewElement{}, err
	}

	// invert order
	for i, j := 0, len(reviews)-1; i < j; i, j = i+1, j-1 {
		reviews[i], reviews[j] = reviews[j], reviews[i]
	}

	hasPrevious := len(reviews) == reviewsPageSize+1
	if hasPrevious {
		reviews = reviews[:reviewsPageSize]
	}

	// compute averages
	// compute averages
	if len(reviews) == 0 {
		return model.CityReviewElement{}, fmt.Errorf("no review found")
	}
	averageLocalTransportRating, averageGreenSpacesRating, averageWasteBinsRating, err := reviewDAO.computeReviewsAverages(reviews[0].CityID)
	if err != nil {
		return model.CityReviewElement{}, fmt.Errorf("error computing average")
	}

	// get number of reviews
	var numReviews int64
	result = db.Model(&model.Review{}).Where("id_city = ?", cityID).Count(&numReviews)
	if result.Error != nil {
		return model.CityReviewElement{}, result.Error
	}

	cityReviewElement := model.CityReviewElement{
		Reviews:                     reviews,
		AverageLocalTransportRating: averageLocalTransportRating,
		AverageGreenSpacesRating:    averageGreenSpacesRating,
		AverageWasteBinsRating:      averageWasteBinsRating,
		HasPrevious:                 hasPrevious,
		HasNext:                     true,
		NumReviews:                  int(numReviews),
	}

	return cityReviewElement, nil
}

func (reviewDAO *ReviewDAO) GetFirstReviewsByCityID(cityID int) (model.CityReviewElement, error) {
	var reviews []model.Review

	// get 11 reviews
	result := db.
		Where("id_city = ?", cityID).
		Order("date_time desc, id_review desc").
		Limit(reviewsPageSize + 1).
		Find(&reviews)
	if result.Error != nil {
		return model.CityReviewElement{}, result.Error
	}

	// inject data
	for i, _ := range reviews {
		err := injectReviewData(&reviews[i])
		if err != nil {
			return model.CityReviewElement{}, err
		}
	}

	// previous and next page
	hasNext := len(reviews) == reviewsPageSize+1
	if hasNext {
		// remove extra element
		reviews = reviews[:reviewsPageSize]
	}

	// compute averages
	if len(reviews) == 0 {
		return model.CityReviewElement{}, fmt.Errorf("no review found")
	}
	averageLocalTransportRating, averageGreenSpacesRating, averageWasteBinsRating, err := reviewDAO.computeReviewsAverages(reviews[0].CityID)
	if err != nil {
		return model.CityReviewElement{}, fmt.Errorf("error computing average")
	}

	// get number of reviews
	var numReviews int64
	result = db.Model(&model.Review{}).Where("id_city = ?", cityID).Count(&numReviews)
	if result.Error != nil {
		return model.CityReviewElement{}, result.Error
	}

	cityReviewElement := model.CityReviewElement{
		Reviews:                     reviews,
		AverageLocalTransportRating: averageLocalTransportRating,
		AverageGreenSpacesRating:    averageGreenSpacesRating,
		AverageWasteBinsRating:      averageWasteBinsRating,
		HasPrevious:                 false,
		HasNext:                     hasNext,
		NumReviews:                  int(numReviews),
	}

	return cityReviewElement, nil
}

func (reviewDAO *ReviewDAO) GetLastReviewsByCityID(cityID int) (model.CityReviewElement, error) {
	// get number of reviews
	var numReviews int64
	result := db.Model(&model.Review{}).Where("id_city = ?", cityID).Count(&numReviews)
	if result.Error != nil {
		return model.CityReviewElement{}, result.Error
	}

	// compute offset
	offset := int(numReviews) - (int(numReviews) % reviewsPageSize)
	if offset == int(numReviews) {
		offset = int(numReviews) - reviewsPageSize
	}

	// get reviews
	var reviews []model.Review
	result = db.Where("id_city = ?", cityID).Order("date_time desc, id_review desc").Offset(offset).Limit(reviewsPageSize).Find(&reviews)
	if result.Error != nil {
		return model.CityReviewElement{}, result.Error
	}

	// inject data
	for i, _ := range reviews {
		err := injectReviewData(&reviews[i])
		if err != nil {
			return model.CityReviewElement{}, err
		}
	}

	hasPrevious := offset > 0

	// compute averages
	if len(reviews) == 0 {
		return model.CityReviewElement{}, fmt.Errorf("no review found")
	}
	averageLocalTransportRating, averageGreenSpacesRating, averageWasteBinsRating, err := reviewDAO.computeReviewsAverages(reviews[0].CityID)
	if err != nil {
		return model.CityReviewElement{}, fmt.Errorf("error computing average")
	}

	cityReviewElement := model.CityReviewElement{
		Reviews:                     reviews,
		AverageLocalTransportRating: averageLocalTransportRating,
		AverageGreenSpacesRating:    averageGreenSpacesRating,
		AverageWasteBinsRating:      averageWasteBinsRating,
		HasPrevious:                 hasPrevious,
		HasNext:                     false,
		NumReviews:                  int(numReviews),
	}

	return cityReviewElement, nil
}

func (reviewDAO *ReviewDAO) computeReviewsAverages(cityId int) (float64, float64, float64, error) {
	var reviewAggregated model.ReviewsAggregated

	err := reviewDAO.db.
		Table("reviews_aggregated").
		Select("*").
		Where("id_city = ?", cityId).
		First(&reviewAggregated)

	if err != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return 0, 0, 0, nil
		} else {
			return 0, 0, 0, err.Error
		}
	}

	if reviewAggregated.NumberRatings == 0 {
		return 0, 0, 0, nil
	}

	averageLocalTransportRating := float64(reviewAggregated.SumLocalTransportRating) / float64(reviewAggregated.NumberRatings)
	averageGreenSpacesRating := float64(reviewAggregated.SumGreenSpacesRating) / float64(reviewAggregated.NumberRatings)
	averageWasteBinsRating := float64(reviewAggregated.SumWasteBinsRating) / float64(reviewAggregated.NumberRatings)

	return averageLocalTransportRating, averageGreenSpacesRating, averageWasteBinsRating, nil
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
	user, err := userDAO.GetUserByIdNoBadges(review.UserID)
	if err != nil {
		return err
	}

	// inject data
	review.CityIata = *city.CityIata
	review.CountryCode = *city.CountryCode
	review.FirstName = user.FirstName
	review.LastName = user.LastName

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
		Select(`
        *,
        (
          (sum_local_transport_rating::FLOAT / NULLIF(number_ratings, 0))
          + (sum_green_spaces_rating::FLOAT    / NULLIF(number_ratings, 0))
          + (sum_waste_bins_rating::FLOAT     / NULLIF(number_ratings, 0))
        ) AS total_average
    `).
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

	var bestReviewsElements []model.CityReviewElement
	for i := 0; i < len(reviewsAggregatedList) && i < bestReviewsNumber; i++ {
		// get reviews
		reviewElement, err1 := reviewDAO.GetFirstReviewsByCityID(reviewsAggregatedList[i].CityID)
		if err1 != nil {
			return nil, err1
		}
		// append to list
		bestReviewsElements = append(bestReviewsElements, reviewElement)
	}

	return bestReviewsElements, nil
}
