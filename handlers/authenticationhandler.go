package handlers

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

func verifyFirebaseToken(ctx context.Context, idToken string) (string, error) {
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
}
