package domain

import "time"

// Vehicle représente un véhicule dans le domaine métier.
type Vehicle struct {
	VehicleID     string
	UserID        string
	Brand         string
	NumberOfSeats int16
	BrandModel    string
	Color         string
	Year          int16 // Année du modèle, 0 = inconnue
	LicencePlate  string
	IsVerified    bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// IsValidVehicleYear accepte 0 (inconnue) ou une année plausible : entre 1950 et
// l'année courante + 1 (millésime suivant commercialisé en fin d'année).
func IsValidVehicleYear(year int16) bool {
	if year == 0 {
		return true
	}
	return year >= 1950 && int(year) <= time.Now().Year()+1
}

// VehiclePreview représente un aperçu réduit d'un véhicule pour les listes.
type VehiclePreview struct {
	VehicleID                 string
	Brand                     string
	BrandModel                string
	LicencePlate              string
	IsVerified                bool
	NumberOfSeats             int16
	AssuranceStatus           string
	VehicleRegistrationStatus string
	DriverLicenceStatus       string
	TripCount                 int32
	Color                     string
	Year                      int16
}

// VehicleDocuments contient les URLs des documents du véhicule (assurance, carte grise)
// ainsi que leurs statuts de revue et les URLs recto/verso du permis du conducteur.
type VehicleDocuments struct {
	AssuranceURL              string
	VehicleRegistrationURL    string
	DriverLicenceRectoURL     string
	DriverLicenceVersoURL     string
	AssuranceStatus           string
	VehicleRegistrationStatus string
}

// VehicleDetails regroupe un véhicule et ses documents associés depuis file-service.
type VehicleDetails struct {
	Vehicle   *Vehicle
	Documents VehicleDocuments
}
