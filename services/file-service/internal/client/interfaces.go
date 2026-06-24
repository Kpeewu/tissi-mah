package client

import "context"

// UserProfile contient les informations d'un utilisateur nécessaires côté file-service.
// Source : user-service.GetUserByFirebaseID.
type UserProfile struct {
	UserID    string // UUID interne MongoDB
	FirstName string
	LastName  string // mappe sur UserProfileResponse.Name
}

// VehicleInfo contient les informations essentielles d'un véhicule.
// Source : vehicle-service.GetVehicleDetails.
type VehicleInfo struct {
	VehicleID    string
	Brand        string
	BrandModel   string
	Color        string
	LicencePlate string
	NumberOfSeats int32
	IsVerified   bool
}

// VehicleClient est l'interface pour communiquer avec vehicle-service via gRPC.
type VehicleClient interface {
	// GetVehicleInfo retourne les informations essentielles d'un véhicule.
	// Retourne nil, nil si le véhicule n'est pas trouvé (dégradation gracieuse).
	GetVehicleInfo(ctx context.Context, vehicleID string) (*VehicleInfo, error)

	// Close libère la connexion gRPC.
	Close() error
}

// UserClient est l'interface pour communiquer avec user-service via gRPC.
// Le file-service reçoit un Firebase UID injecté par l'api-gateway dans la
// metadata gRPC (x-firebase-uid) et doit le résoudre en UUID interne avant
// de stocker les documents (convention partagée avec user-service et kyc-service).
type UserClient interface {
	// GetInternalUserIDByFirebaseID résout un Firebase UID en UserID interne MongoDB.
	GetInternalUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)

	// GetUserProfileByFirebaseID retourne l'UUID interne + prenom/nom.
	// Utilisé par UploadVehicleDocuments pour construire le docName humain-lisible.
	GetUserProfileByFirebaseID(ctx context.Context, firebaseUID string) (*UserProfile, error)

	// Close libère la connexion gRPC.
	Close() error
}
