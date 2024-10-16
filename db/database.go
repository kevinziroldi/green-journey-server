package db

import (
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
)

var db *gorm.DB

func InitDB() (*gorm.DB, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	user := os.Getenv("DB_USERNAME")
	password := os.Getenv("DB_PASSWORD")

	dsn := "host=localhost user=" + user + " password=" + password + " dbname=green_journey_db port=5432 sslmode=disable"

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

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
