package model

type Travel struct {
	TravelID       int     `gorm:"column:id_travel;primaryKey;autoIncrement" json:"travel_id"`
	CO2Compensated float64 `gorm:"column:co2_compensated;type:numeric;not null" json:"co2_compensated"`
	Confirmed      bool    `gorm:"column:confirmed;type:bool;not null" json:"confirmed"`
	UserID         int     `gorm:"column:id_user;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user_id"`
	UserReview     *Review `gorm:"-" json:"user_review"`
}

func (Travel) TableName() string {
	return "travel"
}
