package domain

import "time"

// Decision représente le résultat d'une décision de modération.
type Decision string

const (
	DecisionApproved Decision = "approved"
	DecisionFlagged  Decision = "flagged"
	DecisionBlocked  Decision = "blocked"
)

// Category représente la catégorie de contenu problématique détecté.
type Category string

const (
	CategoryNone      Category = "none"
	CategoryHateSpeech Category = "hate_speech"
	CategoryObscene   Category = "obscene"
	CategoryInsults   Category = "insults"
	CategorySpam      Category = "spam"
	CategoryNSFW      Category = "nsfw"
	CategoryGore      Category = "gore"
	CategoryCSAM      Category = "csam"
)

// ModerationResult est le résultat d'une analyse de modération.
type ModerationResult struct {
	Decision    Decision
	Category    Category
	Score       float32
	Reason      string
	UsedFallback bool
}

// ModerationLog enregistre chaque décision pour audit.
type ModerationLog struct {
	LogID       string
	ContentID   string
	ContentType string
	AuthorID    string
	Decision    Decision
	Category    Category
	Score       float32
	Reason      string
	UsedFallback bool
	CreatedAt   time.Time
}

// UserViolation enregistre une infraction de l'utilisateur (image obscène).
type UserViolation struct {
	ViolationID     string
	UserID          string
	ContentType     string
	ViolationCount  int
	SuspensionUntil *time.Time
	IsPermanentBan  bool
	CreatedAt       time.Time
}

// SuspensionDuration retourne la durée de suspension pour une infraction donnée.
// violationCount est le numéro de l'infraction (1-based).
func SuspensionDuration(violationCount int) (suspendedUntil *time.Time, isBanned bool) {
	now := time.Now().UTC()
	switch violationCount {
	case 1:
		t := now.Add(15 * time.Minute)
		return &t, false
	case 2:
		t := now.Add(24 * time.Hour)
		return &t, false
	case 3:
		t := now.Add(90 * 24 * time.Hour)
		return &t, false
	default:
		return nil, true
	}
}
