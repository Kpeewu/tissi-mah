package interfaces

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/domain"
)

// ChatThreadRepository gère la persistance des threads de conversation.
type ChatThreadRepository interface {
	Create(ctx context.Context, thread *domain.ChatThread) (*domain.ChatThread, error)
	GetByID(ctx context.Context, threadID string) (*domain.ChatThread, error)
	GetByBookingID(ctx context.Context, bookingID string) (*domain.ChatThread, error)
	// GetByUserID retourne tous les threads actifs ou récents d'un utilisateur
	// (en tant que driver ou passenger), triés par updated_at DESC.
	GetByUserID(ctx context.Context, userID string, limit int, afterThreadID string) ([]*domain.ChatThread, error)
	UpdateStatus(ctx context.Context, threadID string, status domain.ThreadStatus, closedAt *time.Time) error
	// CloseByTripID ferme tous les threads actifs d'un trajet donné.
	CloseByTripID(ctx context.Context, tripID string) (int, error)

	// AnonymizeUserRefs pseudonymise driver_id et passenger_id pour les threads de l'utilisateur.
	// Les threads actifs de l'utilisateur sont d'abord fermés.
	AnonymizeUserRefs(ctx context.Context, userID string) error
}

// ChatMessageRepository gère la persistance des messages chiffrés.
type ChatMessageRepository interface {
	Create(ctx context.Context, msg *domain.ChatMessage) (*domain.ChatMessage, error)
	GetByID(ctx context.Context, messageID string) (*domain.ChatMessage, error)
	// GetByThreadID retourne les messages d'un thread depuis un curseur,
	// triés par created_at ASC. Si afterMessageID est vide, retourne les
	// `limit` plus récents.
	GetByThreadID(ctx context.Context, threadID string, afterMessageID string, limit int) ([]*domain.ChatMessage, error)
	// GetLastMessage retourne le dernier message d'un thread (preview).
	GetLastMessage(ctx context.Context, threadID string) (*domain.ChatMessage, error)
	// CountUnread compte les messages non lus depuis la dernière lecture de userID.
	CountUnread(ctx context.Context, threadID string, userID string) (int32, error)
	// MarkReadUntil marque tous les messages jusqu'à messageID comme lus pour userID.
	MarkReadUntil(ctx context.Context, threadID string, userID string, messageID string) error
	Flag(ctx context.Context, messageID string, flaggedBy string) error
	// DeleteOlderThan supprime les messages non signalés plus vieux que cutoff
	// (pour le CronJob de rétention 6 mois).
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)

	// AnonymizeUserRefs pseudonymise sender_id pour les messages de l'utilisateur.
	AnonymizeUserRefs(ctx context.Context, userID string) error
}
