package main

import (
	"flag"
	"github.com/joho/godotenv"
	"green-journey-server/db"
	"green-journey-server/externals"
	"green-journey-server/handlers"
	"green-journey-server/mockservers"
	"log"
	"os"
)

func main() {
	// retrieve execution mode
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	testMode := os.Getenv("TEST_MODE")

	// get port from flag
	port := flag.String("port", "80", "Port on which the server listens")
	flag.Parse()

	// init db
	database, err := db.InitDB(testMode)
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
	SetupRoutes(*port)
}
