package domain

import "time"

// ThreadStatus représente le cycle de vie d'un thread de conversation.
type ThreadStatus string

const (
	ThreadStatusActive ThreadStatus = "active"
	ThreadStatusClosed ThreadStatus = "closed"
)

// SenderRole distingue les participants au sein d'un thread.
type SenderRole string

const (
	SenderRoleDriver    SenderRole = "driver"
	SenderRolePassenger SenderRole = "passenger"
)

// ChatThread est la conversation entre un passager et un chauffeur, liée à
// une réservation spécifique. Un thread est créé à la première demande d'un
// des participants et se ferme automatiquement quand le trajet est terminé.
//
// DriverID et PassengerID sont des UserIDs internes (résolus via
// user-service.GetUserByFirebaseID), JAMAIS le Firebase UID.
// ThreadID est généré côté application (uuid.New) avant l'INSERT.
type ChatThread struct {
	ThreadID    string
	BookingID   string
	TripID      string
	DriverID    string // UserID interne
	PassengerID string // UserID interne
	Status      ThreadStatus
	CreatedAt   time.Time
	ClosedAt    *time.Time
	UpdatedAt   time.Time
}

func (t *ChatThread) IsClosed() bool {
	return t.Status == ThreadStatusClosed
}

// ChatMessage est un message chiffré AES-256-GCM dans un thread.
// Le contenu en clair n'est jamais stocké — seul ContentEncrypted l'est.
// HasRedaction indique si du contenu PII a été masqué (★★★) dans le
// texte livré aux participants.
//
// SenderID et FlaggedBy sont des UserIDs internes (résolus depuis le
// Firebase UID via user-service au moment de l'opération), JAMAIS le
// Firebase UID. MessageID est généré côté application (uuid.New).
type ChatMessage struct {
	MessageID         string
	ThreadID          string
	SenderID          string     // UserID interne
	SenderRole        SenderRole
	ContentEncrypted  []byte     // AES-256-GCM ciphertext
	ContentNonce      []byte     // 12 bytes GCM nonce
	HasRedaction      bool       // PII détecté et masqué
	Flagged           bool
	FlaggedBy         *string    // UserID interne du signaleur
	FlaggedAt         *time.Time
	CreatedAt         time.Time
	ReadAt            *time.Time
}

// UnreadCount est utilisé pour enrichir les previews de threads.
type UnreadCount struct {
	ThreadID    string
	UnreadCount int32
}
