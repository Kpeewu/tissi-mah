package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/domain"
)

type GetOrCreateThreadInput struct {
	UserID    string // Firebase UID de l'appelant
	BookingID string
}

type SendMessageInput struct {
	UserID      string // Firebase UID de l'expéditeur
	ThreadID    string
	Content     string // texte brut; le filtre PII est appliqué dans la couche service
	ForceFlagged bool  // true si la modération a signalé le message
}

type SendMessageResult struct {
	Message      *domain.ChatMessage
	HasRedaction bool
}

type GetMessagesInput struct {
	UserID         string
	ThreadID       string
	AfterMessageID string
	Limit          int
}

type GetMessagesResult struct {
	Messages   []*domain.ChatMessage
	HasMore    bool
	UnreadCount int32
}

type GetUserThreadsInput struct {
	UserID        string
	Limit         int
	AfterThreadID string
}

type ChatService interface {
	GetOrCreateThread(ctx context.Context, input GetOrCreateThreadInput) (*domain.ChatThread, error)
	SendMessage(ctx context.Context, input SendMessageInput) (*SendMessageResult, error)
	GetMessages(ctx context.Context, input GetMessagesInput) (*GetMessagesResult, error)
	GetUserThreads(ctx context.Context, input GetUserThreadsInput) ([]*domain.ChatThread, error)
	MarkRead(ctx context.Context, userID string, threadID string, lastReadMessageID string) error
	FlagMessage(ctx context.Context, userID string, messageID string, reason string) error
	// GetFlaggedMessageContent déchiffre et retourne le contenu d'un message
	// signalé. Uniquement accessible au support (vérification externe).
	GetFlaggedMessageContent(ctx context.Context, messageID string) (string, *domain.ChatMessage, error)
	// CloseThreadsByTripID ferme tous les threads actifs d'un trajet.
	// Appelé par le TripCloserWorker sur event Redis trip.completed.
	CloseThreadsByTripID(ctx context.Context, tripID string) (int, error)

	// AnonymizeUserData ferme les threads actifs de l'utilisateur et pseudonymise
	// ses références dans chat_threads et chat_messages.
	AnonymizeUserData(ctx context.Context, userID string) error
}
