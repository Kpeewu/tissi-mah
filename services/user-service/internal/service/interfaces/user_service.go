package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
)

// FullProfile contient le profil utilisateur enrichi avec les données auth
type FullProfile struct {
	AuthID                     string
	UserID                     string
	Name                       string
	FirstName                  string
	Gender                     string
	DateOfBirth                string
	Bio                        string
	Email                      string
	PhoneNumber                string
	ProfileImageURL            string
	HasProfileImage            bool
	IsDriver                   bool
	IsPassenger                bool
	IsDriverProfileVerified    bool
	IsPassengerProfileVerified bool
	IsActive                   bool
	IsSuspended                bool
	SuspensionEndDate          string
	TripPreferences            []domain.TripPreference
	IDCardExpirationDate       string
	DriveLicenceExpirationDate string
	WithdrawNumber             string
}

// UpdateProfileRequest contient les champs à mettre à jour (optionnels).
// L'utilisateur cible est résolu depuis le Firebase UID du contexte — jamais
// depuis un UserID fourni par le client.
type UpdateProfileRequest struct {
	FirstName      *string
	LastName       *string
	BirthDate      *string
	Email          *string
	PhoneNumber    *string
	WithdrawNumber *string
	Bio            *string
}

type UserService interface {
	// Inter-service (appelés par auth-service / trips-service)
	CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string, birthDate string) (*domain.User, error)
	GetUserByAuthID(ctx context.Context, authID string) (*domain.User, error)
	GetUserByFirebaseID(ctx context.Context, firebaseID string) (*domain.User, error)
	GetUserByUserID(ctx context.Context, userID string) (*domain.User, error)
	GetUsersByUserIDs(ctx context.Context, userIDs []string) ([]*domain.User, error)
	GetUserProfileByUserID(ctx context.Context, userID string) (*domain.User, string, string, error) // user, email, phoneNumber, error

	SoftDeleteUser(ctx context.Context, authID string) error
	// UpdateProfileVerification retourne les valeurs PRÉCÉDENTES des flags
	// (détection des bascules false→true côté kyc-service).
	UpdateProfileVerification(ctx context.Context, userID string, driver, passenger *bool) (prevDriver, prevPassenger bool, err error)

	// Client-facing — l'utilisateur est résolu depuis le Firebase UID du contexte
	// (middleware.FirebaseIDKey), jamais depuis un UserID du body.
	GetMyProfile(ctx context.Context) (*FullProfile, error)
	CreateDriverAccount(ctx context.Context, createDriver bool) error
	AddTripPreferences(ctx context.Context, preferences []domain.TripPreference) error
	UpdateProfile(ctx context.Context, req UpdateProfileRequest) (*FullProfile, error)
}
