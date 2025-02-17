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
	} else if distance >= distanceMidLimit {
		return model.BadgeDistanceMid, nil
	} else if distance >= distanceLowLimit {
		return model.BadgeDistanceLow, nil
	} else {
		return model.BadgeDistanceLow, fmt.Errorf("no badge")
	}
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
	} else if ecologicalChoiceValue >= ecologicalChoiceMidLimit {
		return model.BadgeEcologicalChoiceMid, nil
	} else if ecologicalChoiceValue >= ecologicalChoiceLowLimit {
		return model.BadgeEcologicalChoiceLow, nil
	} else {
		return model.BadgeEcologicalChoiceLow, fmt.Errorf("no badge")
	}
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
	} else if compensationValue >= compensationMidLimit {
		return model.BadgeCompensationMid, nil
	} else if compensationValue >= compensationLowLimit {
		return model.BadgeCompensationLow, nil
	} else {
		return model.BadgeCompensationLow, fmt.Errorf("no badge")
	}
}

func ComputeTravelsNumberCoefficient(numTravels int) (model.Badge, error) {
	if numTravels >= numTravelsHighLimit {
		return model.BadgeTravelsNumberHigh, nil
	} else if numTravels >= numTravelsMidLimit {
		return model.BadgeTravelsNumberMid, nil
	} else if numTravels >= numTravelsLowLimit {
		return model.BadgeTravelsNumberLow, nil
	} else {
		return model.BadgeTravelsNumberLow, fmt.Errorf("no badge")
	}
}
