package db

import (
	"fmt"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
)

var db *gorm.DB
var testMode string

func InitDB(testModeArg string) (*gorm.DB, error) {
	// save testMode
	testMode = testModeArg

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

func CloseDBConnection() {
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed closing connection: ", err)
	}
	err = sqlDB.Close()
	if err != nil {
		log.Fatal("Failed closing connection: ", err)
	}
}

func ResetTestDatabase() error {
	// check correct test mode
	if testMode != "test" {
		return fmt.Errorf("wrong test mode")
	}

	// "user" because it is a reserved word in PostgreSQL
	// don't delete cities in the city table, loaded from dataset
	err := db.Exec(`TRUNCATE TABLE review, reviews_aggregated, segment, travel, "user" CASCADE;`)

	if err.Error != nil {
		return err.Error
	}

	return nil
}
