package db

import (
	"gorm.io/gorm"
	"green-journey-server/model"
	"time"
)

type RankingDao struct {
	db *gorm.DB
}

func NewRankingDAO(db *gorm.DB) *RankingDao {
	return &RankingDao{db: db}
}

func (rankingDAO *RankingDao) ComputeShortDistanceRanking(userID int) ([]model.RankingElement, error) {
	var topUsers []model.User

	// get top 10 users according to short distance score
	err := rankingDAO.db.Order("score_short_distance DESC").Limit(10).Find(&topUsers).Error

	// add requesting user if not present
	topUsers, err = addCurrentUser(topUsers, userID)
	if err != nil {
		return nil, err
	}

	// inject badges
	userDAO := NewUserDAO(GetDB())
	for i, _ := range topUsers {
		err = userDAO.InjectBadges(&topUsers[i])
	}

	topRankingElements := []model.RankingElement{}
	for _, topUser := range topUsers {
		rankingElement, err1 := computeRankingElement(topUser)
		if err1 != nil {
			return nil, err1
		}

		topRankingElements = append(topRankingElements, rankingElement)
	}

	return topRankingElements, nil
}

func (rankingDAO *RankingDao) ComputeLongDistanceRanking(userID int) ([]model.RankingElement, error) {
	var topUsers []model.User

	// get top 10 users according to long distance score
	err := rankingDAO.db.Order("score_long_distance DESC").Limit(10).Find(&topUsers).Error

	// add requesting user if not present
	topUsers, err = addCurrentUser(topUsers, userID)
	if err != nil {
		return nil, err
	}

	// inject badges
	userDAO := NewUserDAO(GetDB())
	for i, _ := range topUsers {
		err = userDAO.InjectBadges(&topUsers[i])
	}

	topRankingElements := []model.RankingElement{}
	for _, topUser := range topUsers {
		rankingElement, err1 := computeRankingElement(topUser)
		if err1 != nil {
			return nil, err1
		}

		topRankingElements = append(topRankingElements, rankingElement)
	}

	return topRankingElements, nil
}

func addCurrentUser(topUsers []model.User, userID int) ([]model.User, error) {
	// check if requesting user present
	found := false
	for _, user := range topUsers {
		if user.UserID == userID {
			found = true
			break
		}
	}

	// add if not present
	if !found {
		// get user
		userDAO := NewUserDAO(GetDB())
		user, err := userDAO.GetUserById(userID)
		if err != nil {
			return nil, err
		}

		// append user
		topUsers = append(topUsers, user)
	}

	return topUsers, nil
}

func computeRankingElement(user model.User) (model.RankingElement, error) {
	// compute values based on user travels
	totalDistance := 0.0
	totalDuration := time.Duration(0)
	totalCO2Emitted := 0.0
	totalCO2Compensated := 0.0

	travelDAO := NewTravelDAO(GetDB())
	travels, err := travelDAO.GetTravelRequestsByUserId(user.UserID)
	if err != nil {
		return model.RankingElement{}, err
	}

	for _, travelDetails := range travels {
		if travelDetails.Travel.Confirmed {
			totalCO2Compensated += travelDetails.Travel.CO2Compensated

			for _, segment := range travelDetails.Segments {
				totalDistance += segment.Distance
				totalDuration += segment.Duration
				totalCO2Emitted += segment.CO2Emitted
			}
		}
	}

	// create and return ranking element
	return model.RankingElement{
		UserID:              user.UserID,
		FirstName:           user.FirstName,
		LastName:            user.LastName,
		ScoreShortDistance:  user.ScoreShortDistance,
		ScoreLongDistance:   user.ScoreLongDistance,
		TotalDistance:       totalDistance,
		TotalDuration:       totalDuration,
		TotalCO2Emitted:     totalCO2Emitted,
		TotalCO2Compensated: totalCO2Compensated,
		Badges:              user.Badges,
	}, nil
}
