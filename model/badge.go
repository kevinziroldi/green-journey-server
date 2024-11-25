package model

import (
	"encoding/json"
	"errors"
)

type Badge int

const (
	BadgeDistanceLow Badge = iota
	BadgeDistanceMid
	BadgeDistanceHigh

	BadgeEcologicalChoiceLow
	BadgeEcologicalChoiceMid
	BadgeEcologicalChoiceHigh

	BadgeCompensationLow
	BadgeCompensationMid
	BadgeCompensationHigh

	BadgeTravelsNumberLow
	BadgeTravelsNumberMid
	BadgeTravelsNumberHigh
)

var badgeStrings = map[Badge]string{
	BadgeDistanceLow:  "badge_distance_low",
	BadgeDistanceMid:  "badge_distance_mid",
	BadgeDistanceHigh: "badge_distance_high",

	BadgeEcologicalChoiceLow:  "badge_ecological_choice_low",
	BadgeEcologicalChoiceMid:  "badge_ecological_choice_mid",
	BadgeEcologicalChoiceHigh: "badge_ecological_choice_high",

	BadgeCompensationLow:  "badge_compensation_low",
	BadgeCompensationMid:  "badge_compensation_mid",
	BadgeCompensationHigh: "badge_compensation_high",

	BadgeTravelsNumberLow:  "badge_travels_number_low",
	BadgeTravelsNumberMid:  "badge_travels_number_mid",
	BadgeTravelsNumberHigh: "badge_travels_number_high",
}

func (b Badge) MarshalJSON() ([]byte, error) {
	if str, ok := badgeStrings[b]; ok {
		return json.Marshal(str)
	}
	return nil, errors.New("invalid badge")
}
