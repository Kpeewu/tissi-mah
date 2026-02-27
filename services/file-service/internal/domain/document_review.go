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

// Types de réviseur valides
var ValidReviewerTypes = map[string]bool{
	"manual":    true,
	"automatic": true,
}

type DocumentReview struct {
	ReviewID          string
	UserDocumentID    *string
	VehicleDocumentID *string

	Decision         string
	ReasonRejection  string
	RejectionDetails string

	ReviewedBy     string
	ReviewedByType string
	ReviewedAt     time.Time

	Notes         string
	ExtractedData json.RawMessage
}

// IsValidReviewDecision vérifie si la décision est valide
func IsValidReviewDecision(decision string) bool {
	return ValidReviewDecisions[decision]
}

// IsValidReviewerType vérifie si le type de réviseur est valide
func IsValidReviewerType(reviewerType string) bool {
	return ValidReviewerTypes[reviewerType]
}
