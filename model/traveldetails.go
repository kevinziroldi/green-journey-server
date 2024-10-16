package model

type TravelDetails struct {
	Travel   Travel    `json:"travel"`
	Segments []Segment `json:"segments"`
}
