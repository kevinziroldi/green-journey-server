package internals

// 15 km/l average fuel efficiency
const fuelEfficiency = 15

func ComputeCarPrice(fuelCostPerLiter, distance, tollCost float64) float64 {
	// fuel cost
	fuelCost := (distance / fuelEfficiency) * fuelCostPerLiter

	// return the sum
	return fuelCost + tollCost
}
