package model

import "time"

type Segment struct {
	SegmentID          int           `gorm:"column:id_segment;primaryKey;autoIncrement" json:"segment_id"`
	DepartureId        int           `gorm:"column:id_departure;type:integer;not null" json:"departure_id"`
	DestinationId      int           `gorm:"column:id_destination;type:integer;not null" json:"destination_id"`
	DepartureCity      string        `gorm:"-" json:"departure_city"`
	DepartureCountry   string        `gorm:"-" json:"departure_country"`
	DestinationCity    string        `gorm:"-" json:"destination_city"`
	DestinationCountry string        `gorm:"-" json:"destination_country"`
	Date               time.Time     `gorm:"column:date;type:date;not null" json:"date"`
	Hour               time.Time     `gorm:"column:hour;type:timestamptz;not null" json:"date_time"`
	Duration           time.Duration `gorm:"column:duration;type:interval;not null" json:"duration"`
	Vehicle            string        `gorm:"column:vehicle;type:text;not null" json:"vehicle"`
	Description        string        `gorm:"column:description;type:text" json:"description"`
	Price              float64       `gorm:"column:price;type:numeric;not null" json:"price"`
	CO2Emitted         float64       `gorm:"column:co2_emitted;type:numeric;not null" json:"co2_emitted"`
	Distance           float64       `gorm:"column:distance;type:numeric;not null" json:"distance"`
	NumSegment         int           `gorm:"column:num_segment;type:integer;not null" json:"num_segment"`
	IsOutward          bool          `gorm:"column:is_outward;type:boolean;not null" json:"is_outward"`
	TravelID           int           `gorm:"column:id_travel;type:integer;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"travel_id"`
}

func (Segment) TableName() string {
	return "segment"
}
