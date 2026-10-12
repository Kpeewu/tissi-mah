package client

import "context"

// UserProfile contient les informations d'un utilisateur nécessaires côté file-service.
// Source : user-service.GetUserByUserID.
type UserProfile struct {
	UserID    string // UUID interne MongoDB
	FirstName string
	LastName  string // mappe sur UserProfileResponse.Name
}

// VehicleInfo contient les informations essentielles d'un véhicule.
// Source : vehicle-service.GetVehicleInfo.
type VehicleInfo struct {
	VehicleID     string
	Brand         string
	BrandModel    string
	Color         string
	LicencePlate  string
	NumberOfSeats int32
	IsVerified    bool
}

// VehicleClient est l'interface pour communiquer avec vehicle-service via gRPC.
type VehicleClient interface {
	// GetVehicleInfo retourne les informations essentielles d'un véhicule.
	// Retourne nil, nil si le véhicule n'est pas trouvé (dégradation gracieuse).
	GetVehicleInfo(ctx context.Context, vehicleID string) (*VehicleInfo, error)

	// GetVehicleOwnerID retourne le UserID interne du propriétaire (contrôle de propriété,
	// sans dégradation gracieuse) : ErrorVehicleNotFound si inconnu,
	// ErrorVehicleServiceUnavailable si vehicle-service est injoignable.
	GetVehicleOwnerID(ctx context.Context, vehicleID string) (string, error)

	// Close libère la connexion gRPC.
	Close() error
}

// UserClient est l'interface pour communiquer avec user-service via gRPC.
// Le file-service résout le Firebase UID en UserID interne dans l'intercepteur ;
// les handlers utilisent directement l'UserID interne issu du contexte.
type UserClient interface {
	// GetUserIDByFirebaseID résout un Firebase UID en UserID interne (intercepteur).
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)

	// GetUserProfileByUserID retourne le profil (UUID interne + prenom/nom) par UserID interne.
	// Utilisé par les handlers d'upload pour construire les métadonnées de document.
	GetUserProfileByUserID(ctx context.Context, userID string) (*UserProfile, error)

	// Close libère la connexion gRPC.
	Close() error
}
