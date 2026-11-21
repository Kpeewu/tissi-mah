package domain

import (
	"bytes"
	"net/http"
	"time"
)

// DetectMimeType complète http.DetectContentType avec un sniff HEIC/HEIF
// car la stdlib ne reconnaît pas ces formats (retourne application/octet-stream).
// Signature ISO/IEC 14496-12 : octets 4-7 = "ftyp", octets 8-11 = brand.
func DetectMimeType(data []byte) string {
	mime := http.DetectContentType(data)
	if mime != "application/octet-stream" {
		return mime
	}
	if len(data) < 12 || !bytes.Equal(data[4:8], []byte("ftyp")) {
		return mime
	}
	switch string(data[8:12]) {
	case "heic", "heix", "hevc", "hevx":
		return "image/heic"
	case "mif1", "msf1", "heim", "heis", "hevm", "hevs":
		return "image/heif"
	}
	return mime
}

// Types de documents utilisateur autorisés.
// "selfie" : photo d'identité de l'utilisateur — devient la photo de profil dès
// l'upload (après modération) et est validée par le support contre la pièce
// d'identité. "profilePicture" est conservé en lecture (historique) mais n'est
// plus écrit : le selfie le remplace.
var ValidUserDocumentTypes = map[string]bool{
	"idCardFront":        true,
	"idCardBack":         true,
	"passport":           true,
	"driverLicenceFront": true,
	"driverLicenceBack":  true,
	"profilePicture":     true,
	"selfie":             true,
}

// ModeratedDocumentTypes : types passant par la modération d'image synchrone
// (images de personnes destinées à être affichées publiquement).
var ModeratedDocumentTypes = map[string]bool{
	"profilePicture": true,
	"selfie":         true,
}

// SelfExemptMetadataTypes : types sans métadonnées légales (pas de numéro,
// dates ni pays — cf. contrainte chk_user_documents_metadata).
var SelfExemptMetadataTypes = map[string]bool{
	"profilePicture": true,
	"selfie":         true,
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
