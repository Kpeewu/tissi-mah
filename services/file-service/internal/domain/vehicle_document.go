package domain

import "time"

// Types de documents véhicule autorisés
var ValidVehicleDocumentTypes = map[string]bool{
	"insurance":        true,
	"registrationCard": true,
}

type VehicleDocument struct {
	DocumentID    string
	UserID        string
	VehicleID     string
	DocumentName  string
	DocumentType  string
	DocumentKey   string
	FileSizeBytes int64
	MimeType      string

	// Informations légales
	DocumentNumber   string
	IssuedAt         *time.Time
	ExpireAt         *time.Time
	IssuingAuthority string

	// Statut
	Status string

	// Remplacement
	IsCurrent  bool
	ReplacedBy *string

	UploadedAt time.Time
	UpdatedAt  time.Time
}

// IsValidVehicleDocumentType vérifie si le type de document véhicule est valide
func IsValidVehicleDocumentType(docType string) bool {
	return ValidVehicleDocumentTypes[docType]
}
