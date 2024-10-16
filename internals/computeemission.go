package internals

func ComputeCarEmission(distance int) float64 {
	return 0.2 * float64(distance)
}

func ComputeAircraftEmission(hours, minutes int) float64 {
	// fuel consumption [tons / hour]
	var fuelConsumption float64
	// co2 emitted [kg co2 / kg fuel]
	const co2Emitted = 3.16
	var availableSeats int
	const percentageTakenSeats = 0.85

	if hours <= 5 {
		// small aircraft
		fuelConsumption = 3
		availableSeats = 150
	} else {
		// big aircraft
		fuelConsumption = 6
		availableSeats = 400
	}

	flightDuration := float64(hours + minutes/60)
	flightEmission := co2Emitted * fuelConsumption * 1000 * flightDuration
	takenSeats := percentageTakenSeats * float64(availableSeats)
	personEmission := flightEmission / takenSeats

	return personEmission
}

func ComputeTrainEmission(distance int) float64 {
	return 0.035 * float64(distance)
}

func ComputeBusEmission(distance int) float64 {
	return 0.03 * float64(distance)
}
