package main

import (
	"green-journey-server/db"
	"green-journey-server/externals"
	"green-journey-server/mockservers"
	"log"
)

func main() {
	// init db
	database, err := db.InitDB()
	if err != nil || database == nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer func() {
		sqlDB, err := database.DB()
		if err != nil {
			log.Println("Failed to get DB from gorm: ", err)
			return
		}
		err = sqlDB.Close()
		if err != nil {
			return
		}
	}()

	// init apis
	externals.InitGoogleMapsApi()
	externals.InitAmadeusApi()

	// start mock servers in new go routines
	go mockservers.StartTollApiServer()
	go mockservers.StartTransitCostApiServer()
	go mockservers.StartFuelCostApiServer()

	// get access token amadeus api
	// TODO must be done (do not remove from main)
	err = externals.GetAccessToken()
	if err != nil {
		log.Fatalf("Failed to get amadeus api access token: %v", err)
		return
	}

	// setup routes
	// TODO must be done (don't remove)
	SetupRoutes()
}
