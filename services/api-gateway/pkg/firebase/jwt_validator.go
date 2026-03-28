package firebase

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	firebaseAuth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// JWTValidator valide les tokens Firebase JWT et extrait le Firebase UID
type JWTValidator struct {
	client *firebaseAuth.Client
}

// NewJWTValidator initialise le validator avec le Firebase Admin SDK.
// Seul le ProjectID est nécessaire : VerifyIDToken vérifie les JWT
// en local avec les clés publiques Google (pas d'appel API, pas de credentials).
func NewJWTValidator(ctx context.Context, projectID string) (*JWTValidator, error) {
	cfg := &firebase.Config{ProjectID: projectID}

	app, err := firebase.NewApp(ctx, cfg, option.WithoutAuthentication())
	if err != nil {
		return nil, fmt.Errorf("firebase: failed to initialize app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase: failed to initialize auth client: %w", err)
	}

	return &JWTValidator{client: client}, nil
}

// VerifyToken valide un Firebase ID token et retourne le Firebase UID (claim "sub").
// Vérifie : signature RS256, issuer, audience, expiration.
func (v *JWTValidator) VerifyToken(ctx context.Context, idToken string) (string, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return "", fmt.Errorf("firebase: invalid token: %w", err)
	}

	return token.UID, nil
}
