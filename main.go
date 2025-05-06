package main

import (
	"context"
	"flag"
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
var port string
var testMode string
var mockOptions bool

func readCommandLineArguments() {
	// read arguments
	portArg := flag.String("port", "80", "Port on which the server listens")
	testModeArg := flag.String("test_mode", "default", "Test mode")
	mockOptionsArg := flag.Bool("mock_options", false, "Mock options")

	flag.Parse()

	port = *portArg
	testMode = *testModeArg
	mockOptions = *mockOptionsArg

	// check valid test mode
	if testMode != "test" && testMode != "real" {
		log.Fatalf("Invalid test mode: %s", testMode)
	}
	log.Println("Test mode:", testMode)
}

func main() {
	// read command line arguments
	readCommandLineArguments()

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
	externals.InitAmadeusApi(mockOptions)

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
	externals.InitializeFirebase(testMode)

	// setup routes and handlers
	server := SetupServer(port)

	// start server
	go func() {
		log.Printf("Server starting on port %s", port)

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
