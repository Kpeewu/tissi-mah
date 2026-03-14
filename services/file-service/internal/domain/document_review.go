package domain

import (
	"encoding/json"
	"time"
)

// Décisions de revue valides
var ValidReviewDecisions = map[string]bool{
	"approved":     true,
	"rejected":     true,
	"resubmission": true,
}

// Types de revue valides (anciennement reviewed_by_type)
var ValidReviewTypes = map[string]bool{
	"manual":    true,
	"automatic": true,
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
	ReviewID          string
	UserDocumentID    *string
	VehicleDocumentID *string

	// Persona
	PersonaInquiryID    string
	PersonaTemplateID   string
	PersonaSessionToken string
	SessionExpiresAt    *time.Time

	// Webhook
	WebhookEventType  string
	WebhookReceivedAt *time.Time
	PersonaRawPayload json.RawMessage

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
	ReviewedAt time.Time

	Notes         string
	ExtractedData json.RawMessage

	// Timestamps
	SubmittedAt *time.Time
	UpdatedAt   time.Time
}

// IsValidReviewDecision vérifie si la décision est valide
func IsValidReviewDecision(decision string) bool {
	return ValidReviewDecisions[decision]
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
