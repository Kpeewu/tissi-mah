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
	LicencePlate  string
	IsVerified    bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
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
