package internals

import (
	"fmt"
	"green-journey-server/model"
)

const distanceLowLimit = 1000  // TODO
const distanceMidLimit = 2000  // TODO
const distanceHighLimit = 3000 // TODO

const ecologicalChoiceLowLimit = 1  // TODO
const ecologicalChoiceMidLimit = 2  // TODO
const ecologicalChoiceHighLimit = 3 // TODO

const compensationLowLimit = 1  // TODO
const compensationMidLimit = 2  // TODO
const compensationHighLimit = 3 // TODO

const numTravelsLowLimit = 5   // TODO
const numTravelsMidLimit = 10  // TODO
const numTravelsHighLimit = 30 // TODO

const ecologicalChoiceCoefficient = 1 // TODO
const compensationCoefficient = 0.12  // TODO

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
	ecologicalChoiceValue := ecologicalChoiceCoefficient * totalDistance / (0.001 + totalCo2Emitted)
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
	compensationValue := compensationCoefficient * totalCO2Compensated / (0.001 + totalCO2Emitted)
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
		return model.BadgeCompensationLow, fmt.Errorf("no badge")
	}
}
