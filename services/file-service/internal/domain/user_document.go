package domain

import "time"

// Types de documents utilisateur autorisés
var ValidUserDocumentTypes = map[string]bool{
	"idCardFront":        true,
	"idCardBack":         true,
	"passport":           true,
	"driverLicenceFront": true,
	"driverLicenceBack":  true,
	"profilePicture":     true,
}

type UserDocument struct {
	DocumentID    string
	UserID        string
	DocumentName  string
	DocumentType  string
	DocumentKey   string
	FileSizeBytes int64
	MimeType      string

	// Informations légales
	DocumentNumber string
	IssuedAt       *time.Time
	ExpireAt       *time.Time
	IssuingCountry string

	// Statut
	Status string

	// Remplacement
	IsCurrent  bool
	ReplacedBy *string

	UploadedAt time.Time
	UpdatedAt  time.Time
}

// IsValidUserDocumentType vérifie si le type de document est valide
func IsValidUserDocumentType(docType string) bool {
	return ValidUserDocumentTypes[docType]
}

// IsValidStatus vérifie si le statut est valide
func IsValidStatus(status string) bool {
	switch status {
	case "pending", "underReview", "approved", "rejected", "expired":
		return true
	}
	return false
}
