package domain

import (
	"encoding/json"
	"time"
)

// Décisions de revue valides
var ValidDecisions = map[string]bool{
	"approved":     true,
	"rejected":     true,
	"resubmission": true,
}

// Statuts de revue valides
var ValidReviewStatuses = map[string]bool{
	"pending":    true,
	"inProgress": true,
	"submitted":  true,
	"completed":  true,
	"expired":    true,
	"failed":     true,
}

// Statuts actifs (empêchent la création d'une nouvelle inquiry)
var ActiveReviewStatuses = map[string]bool{
	"pending":    true,
	"inProgress": true,
}

// IsValidDecision vérifie si la décision est valide
func IsValidDecision(decision string) bool {
	return ValidDecisions[decision]
}

// IsActiveStatus vérifie si le statut bloque la création d'une nouvelle inquiry
func IsActiveStatus(status string) bool {
	return ActiveReviewStatuses[status]
}

// Review représente une revue de document telle que retournée par le file-service
type Review struct {
	ReviewID          string
	UserDocumentID    string
	VehicleDocumentID string

	PersonaInquiryID    string
	PersonaTemplateID   string
	PersonaSessionToken string
	SessionExpiresAt    *time.Time

	WebhookEventType  string
	WebhookReceivedAt *time.Time
	PersonaRawPayload json.RawMessage

	AttemptNumber    int32
	PreviousReviewID string

	Status           string
	Decision         string
	ReasonRejection  string
	RejectionDetails string

	ReviewedBy string
	ReviewType string
	ReviewedAt *time.Time

	Notes         string
	ExtractedData json.RawMessage

	SubmittedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PersonaInquiry représente la réponse de l'API Persona lors de la création d'une inquiry
type PersonaInquiry struct {
	InquiryID    string
	TemplateID   string
	SessionToken string
	ExpiresAt    time.Time
}

// PersonaSession représente un renouvellement de session Persona
type PersonaSession struct {
	SessionToken string
	ExpiresAt    time.Time
}

// DocumentRef contient l'identifiant d'un document retourné par le file-service
type DocumentRef struct {
	DocumentID   string
	DocumentType string
}

// PendingReview est une vue allégée pour le statut KYC
type PendingReview struct {
	ReviewID         string
	PersonaInquiryID string
	Status           string
	AttemptNumber    int32
	SessionExpiresAt *time.Time
}

// LatestRejection contient les informations du dernier rejet
type LatestRejection struct {
	ReviewID         string
	ReasonRejection  string
	RejectionDetails string
	ReviewType       string
	ReviewedAt       *time.Time
}
