package internals

import "math"

func ComputeCarEmission(distance int) float64 {
	return 0.2 * float64(distance)
}

func ComputeAircraftEmission(hours, minutes int) float64 {
	durationMin := float64(hours*60 + minutes)

	// y = 0,000000002163511 x4 - 0,000003861958034 x3 + 0,001920067332020 x2 + 0,410217102378141 x + 20,868891633418000
	return 0.000000002163511*math.Pow(durationMin, 4.0) - 0.000003861958034*math.Pow(durationMin, 3.0) + 0.001920067332020*math.Pow(durationMin, 2.0) + 0.410217102378141*durationMin + 20.868891633418000
}

func ComputeTrainEmission(distance int) float64 {
	return 0.035 * float64(distance)
}

func ComputeBusEmission(distance int) float64 {
	return 0.03 * float64(distance)
}
