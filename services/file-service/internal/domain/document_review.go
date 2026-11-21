package domain

import (
	"encoding/json"
	"time"
)

// Décisions de revue valides
// "pending" = état initial à la création de la review (upload — pas encore de
// décision support). La décision évolue vers approved / rejected / resubmission
// quand un agent support tranche (ValidateDocument / OverrideReview).
var ValidReviewDecisions = map[string]bool{
	"pending":      true,
	"approved":     true,
	"rejected":     true,
	"resubmission": true,
}

// FinalReviewDecisions : décisions qu'un agent support peut prononcer.
// "pending" en est exclu — c'est un état initial, pas une décision.
var FinalReviewDecisions = map[string]bool{
	"approved":     true,
	"rejected":     true,
	"resubmission": true,
}

// Types de revue valides. "automatic" (ex-Persona) est toléré en lecture pour
// les lignes historiques ; seul "manual" est écrit désormais.
var ValidReviewTypes = map[string]bool{
	"manual":    true,
	"automatic": true,
}

// Statuts de revue valides. "inProgress" / "submitted" (ex-Persona) sont
// tolérés en lecture ; plus aucun écrivain depuis l'abandon de Persona.
var ValidReviewStatuses = map[string]bool{
	"pending":    true,
	"inProgress": true,
	"submitted":  true,
	"completed":  true,
	"expired":    true,
	"failed":     true,
}

// Raisons de rejet valides
var ValidReasonRejections = map[string]bool{
	"document_expired":      true,
	"document_incomplete":   true,
	"document_illegible":    true,
	"photo_missmatch":       true,
	"information_missmatch": true,
	"wrong_document_type":   true,
	"other":                 true,
}

type DocumentReview struct {
	ReviewID             string
	UserID               string
	DocumentType         string
	LogicalDocumentType  string // type logique : idCard, driverLicence, passport…
	UserDocumentID       *string
	SecondUserDocumentID *string // verso pour les documents recto-verso
	VehicleDocumentID    *string

	// Retry / versioning
	AttemptNumber    int16
	PreviousReviewID *string

	// Décision
	Status           string
	Decision         string
	ReasonRejection  string
	RejectionDetails string

	// Réviseur
	ReviewedBy string
	ReviewType string
	ReviewedAt *time.Time // nil tant qu'aucune décision n'a été prise

	Notes         string
	ExtractedData json.RawMessage

	// Timestamps
	SubmittedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsValidReviewDecision vérifie si la décision est valide.
// La chaîne vide est acceptée : une review fraîchement créée (status "pending")
// n'a pas encore de décision — elle sera renseignée à la décision support.
func IsValidReviewDecision(decision string) bool {
	if decision == "" {
		return true
	}
	return ValidReviewDecisions[decision]
}

// IsFinalReviewDecision vérifie qu'une décision est prononçable par un agent
// support (approved / rejected / resubmission — jamais "pending").
func IsFinalReviewDecision(decision string) bool {
	return FinalReviewDecisions[decision]
}

// IsValidReviewType vérifie si le type de revue est valide
func IsValidReviewType(reviewType string) bool {
	return ValidReviewTypes[reviewType]
}

// IsValidReviewStatus vérifie si le statut de revue est valide
func IsValidReviewStatus(status string) bool {
	return ValidReviewStatuses[status]
}

// IsValidReasonRejection vérifie si la raison de rejet est valide
func IsValidReasonRejection(reason string) bool {
	if reason == "" {
		return true
	}
	return ValidReasonRejections[reason]
}

// ToLogicalDocumentType dérive le type logique d'un document depuis son type physique.
// idCardFront/idCardBack → idCard ; driverLicenceFront/driverLicenceBack → driverLicence ;
// les autres types (passport, …) sont déjà logiques.
func ToLogicalDocumentType(documentType string) string {
	switch documentType {
	case "idCardFront", "idCardBack":
		return "idCard"
	case "driverLicenceFront", "driverLicenceBack":
		return "driverLicence"
	default:
		return documentType
	}
}

// CompanionDocumentType retourne le type physique de la face complémentaire
// d'un document recto-verso (idCard, driverLicence). Chaîne vide si le type
// est déjà logique/à face unique (passport, insurance, registrationCard…).
func CompanionDocumentType(documentType string) string {
	switch documentType {
	case "idCardFront":
		return "idCardBack"
	case "idCardBack":
		return "idCardFront"
	case "driverLicenceFront":
		return "driverLicenceBack"
	case "driverLicenceBack":
		return "driverLicenceFront"
	default:
		return ""
	}
}
