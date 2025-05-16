package internals

import (
	"fmt"
	"green-journey-server/model"
)

// travel coefficient depends on the vehicle
const TravelCoefficientTransit = 0.44 // train and bus
const TravelCoefficientCar = 0.05
const TravelCoefficientPlane = 0.05
const TravelCoefficientBike = 0.05

const CompensationCoefficient = 0.12
const BonusScore = 2.0

// travels <= 800 km are short, > 800 km are long
const distanceBoundary = 800

func ComputeDeltaScoreModify(travelDetails model.TravelDetails, co2Compensated float64, confirmed bool) (float64, bool, error) {
	travel := travelDetails.Travel
	deltaScore := 0.0

	// compute total distance and co2 emitted
	totalDistance := 0.0
	totalCO2Emitted := 0.0
	for _, segment := range travelDetails.Segments {
		totalDistance += segment.Distance
		totalCO2Emitted += segment.CO2Emitted
	}

	var isShortDistance bool
	if totalDistance <= distanceBoundary {
		isShortDistance = true
	} else {
		isShortDistance = false
	}

	travelCoefficient, err := computeTravelCoefficient(travelDetails)
	if err != nil {
		return 0, true, nil
	}

	if !travel.Confirmed && confirmed {
		if totalCO2Emitted == 0 {
			deltaScore += travelCoefficient * totalDistance
		} else {
			deltaScore += travelCoefficient * totalDistance / totalCO2Emitted
		}
	}

	if travel.CO2Compensated < co2Compensated {
		deltaScore += CompensationCoefficient * (co2Compensated - travel.CO2Compensated)

		if co2Compensated == totalCO2Emitted {
			deltaScore += BonusScore
		}
	}

	return deltaScore, isShortDistance, nil
}

func ComputeDeltaScoreDelete(travelDetails model.TravelDetails) (float64, bool, error) {
	travel := travelDetails.Travel
	deltaScore := 0.0

	if !travelDetails.Travel.Confirmed {
		// score was not added yet
		// no matter short or long distance
		return 0, true, nil
	}

	// compute total distance and co2 emitted
	totalDistance := 0.0
	totalCO2Emitted := 0.0
	for _, segment := range travelDetails.Segments {
		totalDistance += segment.Distance
		totalCO2Emitted += segment.CO2Emitted
	}

	var isShortDistance bool
	if totalDistance <= distanceBoundary {
		isShortDistance = true
	} else {
		isShortDistance = false
	}

	travelCoefficient, err := computeTravelCoefficient(travelDetails)
	if err != nil {
		return 0, true, nil
	}

	if totalCO2Emitted == 0 {
		deltaScore += travelCoefficient * totalDistance
	} else {
		deltaScore += travelCoefficient * totalDistance / totalCO2Emitted
	}
	deltaScore += CompensationCoefficient * travel.CO2Compensated
	if travel.CO2Compensated == totalCO2Emitted {
		deltaScore += BonusScore
	}

	return deltaScore, isShortDistance, nil
}

func computeTravelCoefficient(travelDetails model.TravelDetails) (float64, error) {
	distanceCar := 0.0
	distanceBike := 0.0
	distancePlane := 0.0
	distanceTrain := 0.0
	distanceBus := 0.0

	totalDistance := 0.0

	for _, segment := range travelDetails.Segments {
		if segment.Vehicle == "car" {
			distanceCar += segment.Distance
			totalDistance += segment.Distance
		} else if segment.Vehicle == "bike" {
			distanceBike += segment.Distance
			totalDistance += segment.Distance
		} else if segment.Vehicle == "plane" {
			distancePlane += segment.Distance
			totalDistance += segment.Distance
		} else if segment.Vehicle == "train" {
			distanceTrain += segment.Distance
			totalDistance += segment.Distance
		} else if segment.Vehicle == "bus" {
			distanceBus += segment.Distance
			totalDistance += segment.Distance
		}
	}

	if totalDistance == 0 {
		return 0, fmt.Errorf("no segment with positive distance")
	} else {
		return (TravelCoefficientCar*distanceCar + TravelCoefficientBike*distanceBike + TravelCoefficientPlane*distancePlane + TravelCoefficientTransit*(distanceTrain+distanceBus)) / totalDistance, nil
	}
}
