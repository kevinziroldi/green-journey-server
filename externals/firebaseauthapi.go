package externals

import (
	"context"
	"firebase.google.com/go/v4"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"log"
	"os"
	"sync"
)

var (
	firebaseApp *firebase.App
	once        sync.Once
)

func InitializeFirebase() *firebase.App {
	once.Do(func() {
		opt := option.WithCredentialsFile("firebaseServiceAccountKey.json")
		app, err := firebase.NewApp(context.Background(), nil, opt)
		if err != nil {
			log.Fatalf("Error initializing Firebase Admin SDK: %v", err)
		}
		firebaseApp = app
	})
	return firebaseApp
}

func VerifyFirebaseToken(ctx context.Context, idToken string) (string, error) {
	// retrieve execution mode
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	testMode := os.Getenv("TEST_MODE")

	if testMode == "real" {
		app := InitializeFirebase()

		authClient, err := app.Auth(ctx)
		if err != nil {
			return "", err
		}

		token, err := authClient.VerifyIDToken(ctx, idToken)
		if err != nil {
			return "", err
		}

		return token.UID, nil
	} else {
		// if test mode, return a fake value
		return "firebase_uid", nil
	}
}
