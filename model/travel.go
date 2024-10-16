package model

type Travel struct {
	TravelID       int     `gorm:"column:id_travel;primaryKey;autoIncrement"`
	CO2Compensated float64 `gorm:"column:co2_compensated;type:numeric;not null"`
	UserID         int     `gorm:"column:id_user;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (Travel) TableName() string {
	return "travel"
}
