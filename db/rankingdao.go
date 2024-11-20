package db

import (
	"gorm.io/gorm"
)

type RankingDao struct {
	db *gorm.DB
}

func NewRankingDAO(db *gorm.DB) *RankingDao {
	return &RankingDao{db: db}
}

func (rankingDAO *RankingDao) ComputeShortDistanceRanking() {
	// TODO
}

func (rankingDAO *RankingDao) ComputeLongDistanceRanking() {
	// TODO
}
