package model

import "time"

type RankingElement struct {
	UserID              int           `json:"user_id"`
	FirstName           string        `json:"first_name"`
	LastName            string        `json:"last_name"`
	ScoreShortDistance  float64       `json:"score_short_distance"`
	ScoreLongDistance   float64       `json:"score_long_distance"`
	TotalDistance       float64       `json:"total_distance"`
	TotalDuration       time.Duration `json:"total_duration"`
	TotalCO2Emitted     float64       `json:"total_co_2_emitted"`
	TotalCO2Compensated float64       `json:"total_co_2_compensated"`
	Badges              []Badge       `json:"badges"`
}
