package externals

import (
	"context"
	"firebase.google.com/go/v4"
	"google.golang.org/api/option"
	"log"
	"sync"
)

var (
	firebaseApp *firebase.App
	once        sync.Once
)
var testMode string

func InitializeFirebase(testModeArg string) *firebase.App {
	once.Do(func() {
		opt := option.WithCredentialsFile("firebaseServiceAccountKey.json")
		app, err := firebase.NewApp(context.Background(), nil, opt)
		if err != nil {
			log.Fatalf("Error initializing Firebase Admin SDK: %v", err)
		}
		firebaseApp = app
	})

	testMode = testModeArg

	return firebaseApp
}

func VerifyFirebaseToken(ctx context.Context, idToken string) (string, error) {
	if testMode == "real" {
		app := InitializeFirebase(testMode)

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
