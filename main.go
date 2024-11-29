package main

import (
	"flag"
	"fmt"
	"green-journey-server/db"
	"green-journey-server/externals"
	"green-journey-server/handlers"
	"green-journey-server/mockservers"
	"log"
)

func main() {
	// get port from flag
	port := flag.String("port", "80", "Port on which the server listens")
	flag.Parse()

	fmt.Println(port)

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
	err = externals.GetAccessToken()
	if err != nil {
		log.Fatalf("Failed to get amadeus api access token: %v", err)
		return
	}

	// initialize firebase
	handlers.InitializeFirebase()

	// setup routes
	//SetupRoutes(*port)

	reviewDAO := db.NewReviewDAO(db.GetDB())
	res, err := reviewDAO.GetBestReviews()
	if err != nil {
		log.Fatalf("Failed to get best reviews: %v", err)
		return
	}
	fmt.Println(res)
}
