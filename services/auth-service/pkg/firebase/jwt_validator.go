package firebase

import (
	"context"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	firebaseAuth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// JWTValidator valide les tokens Firebase JWT et extrait le Firebase UID
type JWTValidator struct {
	client *firebaseAuth.Client
}

// NewJWTValidator initialise le validator avec le Firebase Admin SDK.
// Si la variable d'environnement FIREBASE_CREDENTIALS contient le JSON du compte
// de service, elle est utilisée directement. Sinon, on tombe sur ADC
// (utile en GKE avec Workload Identity).
func NewJWTValidator(ctx context.Context, projectID string) (*JWTValidator, error) {
	cfg := &firebase.Config{ProjectID: projectID}

	var opts []option.ClientOption
	if credJSON := os.Getenv("FIREBASE_CREDENTIALS"); credJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(credJSON)))
	}

	app, err := firebase.NewApp(ctx, cfg, opts...)
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
