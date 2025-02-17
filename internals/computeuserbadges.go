package internals

import (
	"fmt"
	"green-journey-server/model"
)

const distanceLowLimit = 3000
const distanceMidLimit = 5000
const distanceHighLimit = 10000

const ecologicalChoiceLowLimit = 15
const ecologicalChoiceMidLimit = 20
const ecologicalChoiceHighLimit = 30

const compensationLowLimit = 0.2
const compensationMidLimit = 0.5
const compensationHighLimit = 0.8

const numTravelsLowLimit = 5
const numTravelsMidLimit = 10
const numTravelsHighLimit = 30

func ComputeDistanceBadge(distance float64) (model.Badge, error) {
	if distance >= distanceHighLimit {
		return model.BadgeDistanceHigh, nil
	}
	if distance >= distanceMidLimit {
		return model.BadgeDistanceMid, nil
	}
	if distance >= distanceLowLimit {
		return model.BadgeDistanceLow, nil
	}
	return model.BadgeDistanceLow, fmt.Errorf("no badge")
}

func ComputeEcologicalChoiceBadge(totalDistance, totalCo2Emitted float64) (model.Badge, error) {
	if totalDistance == 0 {
		return model.BadgeEcologicalChoiceLow, nil
	}
	if totalCo2Emitted == 0 {
		// distance > 0
		return model.BadgeEcologicalChoiceHigh, nil
	}

	ecologicalChoiceValue := totalDistance / totalCo2Emitted
	if ecologicalChoiceValue >= ecologicalChoiceHighLimit {
		return model.BadgeEcologicalChoiceHigh, nil
	}
	if ecologicalChoiceValue >= ecologicalChoiceMidLimit {
		return model.BadgeEcologicalChoiceMid, nil
	}
	if ecologicalChoiceValue >= ecologicalChoiceLowLimit {
		return model.BadgeEcologicalChoiceLow, nil
	}
	return model.BadgeEcologicalChoiceLow, fmt.Errorf("no badge")
}

func ComputeCompensationBadge(totalCO2Compensated, totalCO2Emitted float64) (model.Badge, error) {
	var compensationValue float64

	if totalCO2Emitted == 0 {
		compensationValue = 0
	} else {
		compensationValue = totalCO2Compensated / totalCO2Emitted
	}

	if compensationValue >= compensationHighLimit {
		return model.BadgeCompensationHigh, nil
	}
	if compensationValue >= compensationMidLimit {
		return model.BadgeCompensationMid, nil
	}
	if compensationValue >= compensationLowLimit {
		return model.BadgeCompensationLow, nil
	}

	return model.BadgeCompensationLow, fmt.Errorf("no badge")
}

func ComputeTravelsNumberCoefficient(numTravels int) (model.Badge, error) {
	if numTravels >= numTravelsHighLimit {
		return model.BadgeTravelsNumberHigh, nil
	}
	if numTravels >= numTravelsMidLimit {
		return model.BadgeTravelsNumberMid, nil
	}
	if numTravels >= numTravelsLowLimit {
		fmt.Println("in")
		return model.BadgeTravelsNumberLow, nil
	}
	return model.BadgeTravelsNumberLow, fmt.Errorf("no badge")
}
