package db

import (
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
)

var db *gorm.DB

func InitDB(testMode string) (*gorm.DB, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	user := os.Getenv("DB_USERNAME")
	password := os.Getenv("DB_PASSWORD")

	var dsn string
	if testMode == "real" {
		dsn = "host=localhost user=" + user + " password=" + password + " dbname=green_journey_db port=5432 sslmode=disable"
	} else if testMode == "test" {
		dsn = "host=localhost user=" + user + " password=" + password + " dbname=green_journey_db_test port=5432 sslmode=disable"
	} else {
		log.Fatal("Invalid test mode")
	}

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt: true,
	})

	if err != nil {
		// can't connect to the db, the server should stop
		log.Fatalf("Failed to connect to database: %v", err)
		return nil, err
	}

	return db, nil
}

func GetDB() *gorm.DB {
	return db
}

func ResetTestDatabase() {
	// retrieve execution mode
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	testMode := os.Getenv("TEST_MODE")

	// already checked by the handler
	// double check, if called by someone else in the future
	// I don't want to delete data from my actual db
	if testMode != "test" {
		return
	}

	// "user" because it is a reserved word in PostgreSQL
	// don't delete cities in the city table, loaded from dataset
	err1 := db.Exec(`TRUNCATE TABLE airport, review, reviews_aggregated, segment, travel, "user" CASCADE;`)

	if err1.Error != nil {
		log.Fatalf("Failed to reset test database: %v", err)
	}
}
