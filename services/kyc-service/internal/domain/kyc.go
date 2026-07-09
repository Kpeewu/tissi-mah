package domain

import (
	"encoding/json"
	"time"
)

// Décisions de revue valides
// "pending" = état initial à la création (pas encore de décision Persona).
var ValidDecisions = map[string]bool{
	"pending":      true,
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
	ReviewID             string
	UserID               string
	DocumentType         string
	LogicalDocumentType  string // idCard, driverLicence, passport…
	UserDocumentID       string
	SecondUserDocumentID string // verso pour les documents recto-verso
	VehicleDocumentID    string

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

// DocumentRef contient l'identifiant d'un document retourné par le file-service.
// OwnerID est l'UUID interne (user_id pour user_documents, vehicle_id pour vehicle_documents)
// — utilisé pour vérifier que le document appartient bien à l'appelant.
// DocumentURL est l'URL S3/MinIO publique — utilisée pour soumettre les
// documents à Persona via SubmitGovernmentID.
type DocumentRef struct {
	DocumentID   string
	DocumentType string
	OwnerID      string
	DocumentURL  string
	// UserID est renseigné uniquement pour les documents véhicule, où OwnerID
	// est le vehicle_id. Pour les documents utilisateur OwnerID est déjà le user_id.
	UserID string
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

// =============================================================================
// Validation manuelle (support) — catégorisation passenger / driver
// =============================================================================

const (
	CategoryPassenger = "passenger"
	CategoryDriver    = "driver"
	CategoryOther     = "other"
)

// passengerDocumentTypes : documents d'identité (validation "passenger").
var passengerDocumentTypes = map[string]bool{
	"idCardFront": true,
	"idCardBack":  true,
	"passport":    true,
}

// driverUserDocumentTypes : documents utilisateur relatifs au permis (validation "driver").
// Le permis est un document recto-verso partagé identité/véhicule.
var driverUserDocumentTypes = map[string]bool{
	"driverLicenceFront": true,
	"driverLicenceBack":  true,
}

// DocumentCategory classe un document en "passenger" / "driver" / "other".
// Tout document véhicule (ownerKind == "vehicle") relève du "driver".
func DocumentCategory(documentType, ownerKind string) string {
	if ownerKind == "vehicle" {
		return CategoryDriver
	}
	if passengerDocumentTypes[documentType] {
		return CategoryPassenger
	}
	if driverUserDocumentTypes[documentType] {
		return CategoryDriver
	}
	return CategoryOther
}

// statusPrecedence ordonne les statuts pour l'agrégation par catégorie : plus la
// valeur est élevée, plus le statut prime (triage : rejected en premier).
var statusPrecedence = map[string]int{
	"rejected":    5,
	"underReview": 4,
	"pending":     3,
	"expired":     2,
	"approved":    1,
}

// AggregateStatus retourne le statut prioritaire d'un ensemble de statuts de documents
// (précédence rejected > underReview > pending > expired > approved). "" si vide.
func AggregateStatus(statuses []string) string {
	best := ""
	bestRank := 0
	for _, s := range statuses {
		if r := statusPrecedence[s]; r > bestRank {
			bestRank = r
			best = s
		}
	}
	return best
}

// KycDocument : document KYC (user ou véhicule) remonté par le file-service.
type KycDocument struct {
	DocumentID   string
	UserID       string
	VehicleID    string
	DocumentType string
	Status       string
	OwnerKind    string // "user" | "vehicle"
	UpdatedAt    string
	UploadedAt   string // ISO 8601 — date de dépôt initial
}

// VehicleDetails : infos véhicule embarquées dans DocumentSummary (vehicle docs uniquement).
type VehicleDetails struct {
	VehicleID     string
	Brand         string
	BrandModel    string
	Color         string
	LicencePlate  string
	NumberOfSeats int32
	IsVerified    bool
}

// DocumentSummary : document soumis par un utilisateur (vue détail support).
type DocumentSummary struct {
	DocumentID          string
	DocumentType        string
	LogicalDocumentType string // idCard, driverLicence, passport…
	Status              string
	OwnerKind           string // "user" | "vehicle"
	OwnerID             string // user_id ou vehicle_id selon OwnerKind
	Category            string // passenger | driver | other
	LatestReview        *ReviewSummary
	// Métadonnées document (recto / document principal)
	DocumentURL      string
	FileSizeBytes    int64
	MimeType         string
	DocumentNumber   string
	IsCurrent        bool
	UploadedAt       string
	UpdatedAt        string
	IssuedAt         string
	ExpiredAt        string          // unifié : ExpiredAt user / ExpireAt vehicle
	IssuingCountry   string          // user docs uniquement
	IssuingAuthority string          // vehicle docs uniquement
	Vehicle          *VehicleDetails // vehicle docs uniquement
	// Verso — recto-verso (idCard, driverLicence) ; vide si document singulier
	SecondDocumentID    string
	SecondDocumentURL   string
	SecondFileSizeBytes int64
	SecondMimeType      string
	SecondUploadedAt    string
	SecondUpdatedAt     string
}

// ReviewSummary : dernière review associée à un document.
type ReviewSummary struct {
	ReviewID         string
	Status           string
	Decision         string
	ReasonRejection  string
	RejectionDetails string
	ReviewType       string
	ReviewedBy       string
	ReviewedAt       *time.Time
	// Identité de l'agent support ayant revu (résolue via support-service)
	ReviewedByFirstName       string
	ReviewedByLastName        string
	ReviewedByProfileImageURL string // réservé — vide tant que les agents n'ont pas de photo
}

// UserInfo : infos profil utilisateur (récupérées via user-service).
type UserInfo struct {
	UserID          string
	Name            string
	FirstName       string
	Email           string
	PhoneNumber     string
	ProfileImageURL string
}

// SupportAgent : infos d'un agent support (récupérées via support-service)
// pour enrichir les reviews avec le prénom/nom de l'agent ayant revu un document.
type SupportAgent struct {
	UserID    string
	FirstName string
	LastName  string
	Role      string
}

// ManualReviewRequest : entrée de la liste groupée par utilisateur.
type ManualReviewRequest struct {
	User            *UserInfo
	PassengerStatus string
	DriverStatus    string
	TotalDocuments  int32
	LastDepositAt   string // ISO 8601 — date du dernier document déposé
}

// ManualReviewRequestDetail : détail d'une demande (user + tous ses documents).
type ManualReviewRequestDetail struct {
	User      *UserInfo
	Documents []*DocumentSummary
}

// DocumentHistoryEntry : entrée de l'historique d'un document logique (vue support).
type DocumentHistoryEntry struct {
	ReviewID            string
	Status              string
	Decision            string
	ReasonRejection     string
	RejectionDetails    string
	Notes               string
	ReviewType          string
	ReviewedBy          string
	ReviewedAt          *time.Time
	AttemptNumber       int32
	DocumentID          string // recto / face principale
	SecondDocumentID    string // verso (vide si non recto-verso)
	LogicalDocumentType string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	// Identité de l'agent support ayant revu (résolue via support-service)
	ReviewedByFirstName       string
	ReviewedByLastName        string
	ReviewedByProfileImageURL string // réservé — vide tant que les agents n'ont pas de photo
}

// ToLogicalDocumentType dérive le type logique depuis le type physique.
// La carte d'identité et le permis sont recto-verso ; les autres types sont déjà logiques.
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

// CompanionDocumentType retourne le type du côté compagnon pour les documents
// recto-verso (idCard, driverLicence). Retourne "" si le type n'est pas recto-verso.
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
