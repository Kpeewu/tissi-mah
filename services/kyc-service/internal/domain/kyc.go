package domain

import (
	"encoding/json"
	"time"
)

// Décisions de revue valides
// "pending" = état initial à la création (pas encore de décision support).
var ValidDecisions = map[string]bool{
	"pending":      true,
	"approved":     true,
	"rejected":     true,
	"resubmission": true,
}

// FinalDecisions : décisions prononçables par un agent support.
// "pending" en est exclu — c'est un état initial, pas une décision : l'accepter
// produirait une review "completed/pending" impossible à re-valider ou overrider.
var FinalDecisions = map[string]bool{
	"approved":     true,
	"rejected":     true,
	"resubmission": true,
}

// Statuts de revue valides ("inProgress"/"submitted" : héritage Persona, lecture seule)
var ValidReviewStatuses = map[string]bool{
	"pending":    true,
	"inProgress": true,
	"submitted":  true,
	"completed":  true,
	"expired":    true,
	"failed":     true,
}

// IsValidDecision vérifie si la décision est valide
func IsValidDecision(decision string) bool {
	return ValidDecisions[decision]
}

// IsFinalDecision vérifie qu'une décision est prononçable par un agent support.
func IsFinalDecision(decision string) bool {
	return FinalDecisions[decision]
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

// DocumentRef contient l'identifiant d'un document retourné par le file-service.
// OwnerID est l'UUID interne (user_id pour user_documents, vehicle_id pour vehicle_documents)
// — utilisé pour vérifier que le document appartient bien à l'appelant.
type DocumentRef struct {
	DocumentID   string
	DocumentType string
	OwnerID      string
	DocumentURL  string
	// UserID est renseigné uniquement pour les documents véhicule, où OwnerID
	// est le vehicle_id. Pour les documents utilisateur OwnerID est déjà le user_id.
	UserID string
	// Status du document (pending/approved/rejected/expired) — utilisé par le
	// calcul de vérification basé documents.
	Status string
}

// PendingReview est une vue allégée pour le statut KYC
type PendingReview struct {
	ReviewID      string
	Status        string
	AttemptNumber int32
	DocumentType  string
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
// Le selfie en fait partie : l'identité exige une pièce approuvée ET un selfie
// approuvé (comparaison visuelle par le support).
var passengerDocumentTypes = map[string]bool{
	"idCardFront": true,
	"idCardBack":  true,
	"passport":    true,
	"selfie":      true,
}

// driverUserDocumentTypes : documents utilisateur relatifs au permis (validation "driver").
// Le permis est un document recto-verso à DOUBLE RÔLE : preuve du droit de
// conduire ET pièce d'identité valable — il appartient donc aux deux catégories.
var driverUserDocumentTypes = map[string]bool{
	"driverLicenceFront": true,
	"driverLicenceBack":  true,
}

// DocumentCategories classe un document dans ses catégories.
// Un document peut appartenir à plusieurs catégories : le permis (double rôle
// explicite) compte à la fois pour "passenger" (pièce d'identité) et "driver".
// profilePicture (héritage) et types inconnus → aucune catégorie (exclus des
// files de validation).
func DocumentCategories(documentType, ownerKind string) []string {
	if ownerKind == "vehicle" {
		return []string{CategoryDriver}
	}
	if driverUserDocumentTypes[documentType] {
		return []string{CategoryPassenger, CategoryDriver}
	}
	if passengerDocumentTypes[documentType] {
		return []string{CategoryPassenger}
	}
	return nil
}

// DocumentCategory retourne la catégorie PRINCIPALE d'un document (affichage) :
// le permis reste étiqueté "driver" (son groupe d'affichage historique) même
// s'il compte aussi côté passenger — cf. DocumentCategories pour la vérité
// multi-catégories.
func DocumentCategory(documentType, ownerKind string) string {
	if ownerKind == "vehicle" || driverUserDocumentTypes[documentType] {
		return CategoryDriver
	}
	if passengerDocumentTypes[documentType] {
		return CategoryPassenger
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
	Category            string   // catégorie principale (affichage) : passenger | driver | other
	Categories          []string // toutes les catégories — le permis = [passenger, driver]
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
	Notes            string
	AttemptNumber    int32
	PreviousReviewID string
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

// VerificationDocument : vue minimale d'un document COURANT pour le calcul de
// vérification (type physique + statut ; VehicleID renseigné pour un document véhicule).
type VerificationDocument struct {
	DocumentType string
	Status       string
	VehicleID    string // vide pour un document utilisateur
}

// ComputeVehicleVerification retourne, par véhicule, si tous ses documents requis
// (assurance + carte grise) sont approuvés. La clé couvre TOUS les véhicules ayant
// au moins un document courant (y compris non vérifiés — pour pouvoir dé-vérifier).
func ComputeVehicleVerification(vehicleDocs []*VerificationDocument) map[string]bool {
	type vState struct{ insurance, registration bool }
	states := make(map[string]*vState)
	for _, d := range vehicleDocs {
		if d.VehicleID == "" {
			continue
		}
		st := states[d.VehicleID]
		if st == nil {
			st = &vState{}
			states[d.VehicleID] = st
		}
		if d.Status != "approved" {
			continue
		}
		switch d.DocumentType {
		case "insurance":
			st.insurance = true
		case "registrationCard":
			st.registration = true
		}
	}
	out := make(map[string]bool, len(states))
	for vehicleID, st := range states {
		out[vehicleID] = st.insurance && st.registration
	}
	return out
}

// ComputeProfileVerification dérive les flags de vérification KYC (passager /
// conducteur) à partir des documents COURANTS de l'utilisateur — plus des reviews :
// resoumission, override et remplacement de selfie sont ainsi couverts uniformément
// (le statut des documents courants est la source de vérité, synchronisé par le
// file-service à chaque décision).
//
// Règles :
//   - Identité (passager) = selfie approuvé ET une pièce approuvée
//     (CNI recto+verso OU passeport OU permis recto+verso — double rôle du permis).
//   - Conducteur = selfie + permis approuvés ET au moins UN véhicule entièrement
//     validé (assurance + carte grise approuvées pour ce même véhicule).
func ComputeProfileVerification(userDocs, vehicleDocs []*VerificationDocument) (identityVerified, driverVerified bool) {
	approved := make(map[string]bool)
	for _, d := range userDocs {
		if d.Status == "approved" {
			approved[d.DocumentType] = true
		}
	}

	selfieOK := approved["selfie"]
	idCardOK := approved["idCardFront"] && approved["idCardBack"]
	licenceOK := approved["driverLicenceFront"] && approved["driverLicenceBack"]
	idProofOK := idCardOK || approved["passport"] || licenceOK

	anyVehicleOK := false
	for _, ok := range ComputeVehicleVerification(vehicleDocs) {
		if ok {
			anyVehicleOK = true
			break
		}
	}

	identityVerified = selfieOK && idProofOK
	driverVerified = selfieOK && licenceOK && anyVehicleOK
	return
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
