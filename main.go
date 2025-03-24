package main

import (
	"context"
	"flag"
	"github.com/joho/godotenv"
	"green-journey-server/db"
	"green-journey-server/externals"
	"green-journey-server/mockservers"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var shutdownTimeout = 10 * time.Second

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
	// defer close db connection
	defer func() {
		db.CloseDBConnection()
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
	externals.InitializeFirebase()

	// setup routes
	server := SetupServer(*port)

	// start server
	go func() {
		log.Printf("Server starting on port %s", *port)

		err = server.ListenAndServeTLS("GreenJourneyServerCertificate.crt", "GreenJourneyServerKey.key")
		if err != nil {
			// fatal condition
			log.Fatalf("Failed to start the server")
		}
	}()

	// graceful termination
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received, shutting down server")
	// set timeout to close connections
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed: %v", err)
	}

	log.Println("Server exited gracefully")
}
