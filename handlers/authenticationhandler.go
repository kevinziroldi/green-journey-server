package handlers

import (
	"context"
	"firebase.google.com/go/v4"
)

func verifyFirebaseToken(ctx context.Context, idToken string) (string, error) {
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		return "", err
	}

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
