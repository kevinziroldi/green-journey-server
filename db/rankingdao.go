package db

import (
	"gorm.io/gorm"
)

type RankingsDao struct {
	db *gorm.DB
}

func NewRankingsDAO(db *gorm.DB) *RankingsDao {
	return &RankingsDao{db: db}
}
