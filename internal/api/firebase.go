package api

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
}

func NewFirebaseVerifier(credentialsFile, credentialsJSON, projectID string) (TokenVerifier, error) {
	var opts []option.ClientOption
	switch {
	case credentialsJSON != "":
		opts = append(opts, option.WithCredentialsJSON([]byte(credentialsJSON)))
	case credentialsFile != "":
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	conf := &firebase.Config{ProjectID: projectID}
	app, err := firebase.NewApp(context.Background(), conf, opts...)
	if err != nil {
		return nil, fmt.Errorf("init firebase app: %w", err)
	}
	client, err := app.Auth(context.Background())
	if err != nil {
		return nil, fmt.Errorf("init firebase auth: %w", err)
	}
	return client, nil
}
