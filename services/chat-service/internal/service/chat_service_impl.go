package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/crypto"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/filter"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/chat-service/internal/repository/interfaces"
	svcInterfaces "github.com/Kpeewu/tissi-mah/services/chat-service/internal/service/interfaces"
	chatErrors "github.com/Kpeewu/tissi-mah/services/chat-service/pkg/errors"
)

const (
	maxMessageLength = 1000 // chars
	defaultPageSize  = 30
	maxPageSize      = 50
)

type chatServiceImpl struct {
	threadRepo    repoInterfaces.ChatThreadRepository
	messageRepo   repoInterfaces.ChatMessageRepository
	userClient    client.UserClient
	bookingClient client.BookingClient
	tripClient    client.TripClient
	encryptor     *crypto.MessageEncryptor
	piiFilter     *filter.PIIFilter
	logger        *zap.Logger
}

func NewChatService(
	threadRepo repoInterfaces.ChatThreadRepository,
	messageRepo repoInterfaces.ChatMessageRepository,
	userClient client.UserClient,
	bookingClient client.BookingClient,
	tripClient client.TripClient,
	encryptor *crypto.MessageEncryptor,
	logger *zap.Logger,
) svcInterfaces.ChatService {
	return &chatServiceImpl{
		threadRepo:    threadRepo,
		messageRepo:   messageRepo,
		userClient:    userClient,
		bookingClient: bookingClient,
		tripClient:    tripClient,
		encryptor:     encryptor,
		piiFilter:     filter.NewPIIFilter(),
		logger:        logger,
	}
}

// resolveInternalUserID convertit un Firebase UID en UserID interne.
// Toutes les opérations chat (autorisation, sender_id, comparaisons avec
// thread.DriverID/PassengerID retournés par booking-service) utilisent
// uniquement le UserID interne — jamais le Firebase UID.
func (s *chatServiceImpl) resolveInternalUserID(ctx context.Context, firebaseUID string) (string, error) {
	if firebaseUID == "" {
		return "", chatErrors.ErrUnauthorized
	}
	internalID, err := s.userClient.GetUserIDByFirebaseID(ctx, firebaseUID)
	if err != nil {
		s.logger.Error("chat: resolve firebase uid to internal user id failed",
			zap.String("firebaseUID", firebaseUID),
			zap.Error(err),
		)
		return "", chatErrors.ErrUnauthorized
	}
	return internalID, nil
}

// GetOrCreateThread ouvre ou retrouve le thread lié à une réservation.
// Vérifie que la réservation est "approved" et que le trajet n'est pas terminé.
// input.UserID est le Firebase UID injecté par l'api-gateway ; il est résolu
// en UserID interne pour comparaison avec booking.DriverID / booking.PassengerID.
func (s *chatServiceImpl) GetOrCreateThread(ctx context.Context, input svcInterfaces.GetOrCreateThreadInput) (*domain.ChatThread, error) {
	if input.BookingID == "" {
		return nil, chatErrors.ErrMissingBookingID
	}

	// Résoudre Firebase UID → UserID interne (toutes les comparaisons
	// d'autorisation se font avec l'UserID interne).
	internalUserID, err := s.resolveInternalUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	// Vérifier que la réservation appartient à l'appelant et est approuvée.
	booking, err := s.bookingClient.GetBookingDetails(ctx, input.BookingID)
	if err != nil {
		s.logger.Error("chat: get booking details failed", zap.Error(err))
		return nil, chatErrors.ErrBookingServiceUnavailable
	}
	if booking == nil {
		return nil, chatErrors.ErrBookingNotFound
	}

	// L'appelant doit être le passager ou le chauffeur de cette réservation.
	if booking.PassengerID != internalUserID && booking.DriverID != internalUserID {
		return nil, chatErrors.ErrUnauthorized
	}

	if booking.Status != "approved" {
		return nil, chatErrors.ErrBookingNotApproved
	}

	// Vérifier que le trajet n'est pas terminé.
	trip, err := s.tripClient.GetTripByID(ctx, booking.TripID)
	if err != nil {
		s.logger.Error("chat: get trip failed", zap.Error(err))
		return nil, chatErrors.ErrTripServiceUnavailable
	}
	if trip.Status == "completed" || trip.Status == "cancelled" {
		return nil, chatErrors.ErrTripAlreadyEnded
	}

	// Retrouver un thread existant pour cette réservation.
	existing, err := s.threadRepo.GetByBookingID(ctx, input.BookingID)
	if err != nil {
		return nil, fmt.Errorf("chat: get thread by booking: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// Créer le thread. ThreadID généré côté back (uuid.New) et envoyé à la
	// DB ; les driver_id / passenger_id sont les UserIDs internes retournés
	// par booking-service (cohérence avec sender_id des messages).
	thread := &domain.ChatThread{
		ThreadID:    uuid.New().String(),
		BookingID:   booking.BookingID,
		TripID:      booking.TripID,
		DriverID:    booking.DriverID,
		PassengerID: booking.PassengerID,
		Status:      domain.ThreadStatusActive,
	}
	created, err := s.threadRepo.Create(ctx, thread)
	if err != nil {
		return nil, fmt.Errorf("chat: create thread: %w", err)
	}
	s.logger.Info("chat: thread created",
		zap.String("threadID", created.ThreadID),
		zap.String("bookingID", input.BookingID),
	)
	return created, nil
}

// SendMessage envoie un message dans un thread. Le filtre PII masque les
// coordonnées directes avant le chiffrement.
func (s *chatServiceImpl) SendMessage(ctx context.Context, input svcInterfaces.SendMessageInput) (*svcInterfaces.SendMessageResult, error) {
	if input.ThreadID == "" {
		return nil, chatErrors.ErrMissingThreadID
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, chatErrors.ErrEmptyMessage
	}
	if len([]rune(content)) > maxMessageLength {
		return nil, chatErrors.ErrMessageTooLong
	}

	// Résoudre Firebase UID → UserID interne (sender_id stocké en DB).
	internalUserID, err := s.resolveInternalUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	// Vérifier le thread et les droits d'accès.
	thread, err := s.threadRepo.GetByID(ctx, input.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("chat: get thread: %w", err)
	}
	if thread == nil {
		return nil, chatErrors.ErrThreadNotFound
	}
	if thread.IsClosed() {
		return nil, chatErrors.ErrThreadClosed
	}

	// Seuls driver et passenger du thread peuvent écrire.
	role, err := s.resolveRole(thread, internalUserID)
	if err != nil {
		return nil, err
	}

	// Appliquer le filtre PII sur le contenu.
	filtered, hasRedaction := s.piiFilter.Filter(content)
	if hasRedaction {
		s.logger.Info("chat: PII redacted in message",
			zap.String("threadID", input.ThreadID),
			zap.String("senderID", internalUserID),
		)
	}

	// Chiffrer le contenu filtré (celui qui sera livré aux participants).
	ciphertext, nonce, err := s.encryptor.Encrypt([]byte(filtered), input.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("chat: encrypt message: %w", err)
	}

	// L'ID est généré côté back (uuid.New) et envoyé en base — pas de DEFAULT
	// gen_random_uuid() côté DB pour rendre l'intention explicite.
	msg := &domain.ChatMessage{
		MessageID:        uuid.New().String(),
		ThreadID:         input.ThreadID,
		SenderID:         internalUserID,
		SenderRole:       role,
		ContentEncrypted: ciphertext,
		ContentNonce:     nonce,
		HasRedaction:     hasRedaction,
	}
	created, err := s.messageRepo.Create(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("chat: save message: %w", err)
	}

	s.logger.Debug("chat: message sent",
		zap.String("messageID", created.MessageID),
		zap.String("threadID", input.ThreadID),
	)
	return &svcInterfaces.SendMessageResult{
		Message:      created,
		HasRedaction: hasRedaction,
	}, nil
}

// GetMessages retourne les messages d'un thread depuis un curseur.
func (s *chatServiceImpl) GetMessages(ctx context.Context, input svcInterfaces.GetMessagesInput) (*svcInterfaces.GetMessagesResult, error) {
	if input.ThreadID == "" {
		return nil, chatErrors.ErrMissingThreadID
	}

	internalUserID, err := s.resolveInternalUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	thread, err := s.threadRepo.GetByID(ctx, input.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("chat: get thread: %w", err)
	}
	if thread == nil {
		return nil, chatErrors.ErrThreadNotFound
	}
	if thread.DriverID != internalUserID && thread.PassengerID != internalUserID {
		return nil, chatErrors.ErrUnauthorized
	}

	limit := input.Limit
	if limit <= 0 || limit > maxPageSize {
		limit = defaultPageSize
	}

	// Fetch un de plus pour savoir s'il y a une page suivante.
	msgs, err := s.messageRepo.GetByThreadID(ctx, input.ThreadID, input.AfterMessageID, limit+1)
	if err != nil {
		return nil, fmt.Errorf("chat: get messages: %w", err)
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	unread, err := s.messageRepo.CountUnread(ctx, input.ThreadID, internalUserID)
	if err != nil {
		s.logger.Warn("chat: count unread failed", zap.Error(err))
	}

	return &svcInterfaces.GetMessagesResult{
		Messages:    msgs,
		HasMore:     hasMore,
		UnreadCount: unread,
	}, nil
}

// GetUserThreads retourne les threads d'un utilisateur.
func (s *chatServiceImpl) GetUserThreads(ctx context.Context, input svcInterfaces.GetUserThreadsInput) ([]*domain.ChatThread, error) {
	internalUserID, err := s.resolveInternalUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	return s.threadRepo.GetByUserID(ctx, internalUserID, limit, input.AfterThreadID)
}

// MarkRead marque les messages comme lus.
func (s *chatServiceImpl) MarkRead(ctx context.Context, userID string, threadID string, lastReadMessageID string) error {
	if threadID == "" || lastReadMessageID == "" {
		return chatErrors.ErrMissingThreadID
	}
	internalUserID, err := s.resolveInternalUserID(ctx, userID)
	if err != nil {
		return err
	}
	thread, err := s.threadRepo.GetByID(ctx, threadID)
	if err != nil {
		return fmt.Errorf("chat: get thread: %w", err)
	}
	if thread == nil {
		return chatErrors.ErrThreadNotFound
	}
	if thread.DriverID != internalUserID && thread.PassengerID != internalUserID {
		return chatErrors.ErrUnauthorized
	}
	return s.messageRepo.MarkReadUntil(ctx, threadID, internalUserID, lastReadMessageID)
}

// FlagMessage signale un message pour modération.
func (s *chatServiceImpl) FlagMessage(ctx context.Context, userID string, messageID string, reason string) error {
	if messageID == "" {
		return chatErrors.ErrMissingMessageID
	}
	internalUserID, err := s.resolveInternalUserID(ctx, userID)
	if err != nil {
		return err
	}
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("chat: get message: %w", err)
	}
	if msg == nil {
		return chatErrors.ErrMessageNotFound
	}
	// Vérifier que l'appelant participe au thread.
	thread, err := s.threadRepo.GetByID(ctx, msg.ThreadID)
	if err != nil {
		return fmt.Errorf("chat: get thread: %w", err)
	}
	if thread == nil || (thread.DriverID != internalUserID && thread.PassengerID != internalUserID) {
		return chatErrors.ErrUnauthorized
	}
	s.logger.Info("chat: message flagged",
		zap.String("messageID", messageID),
		zap.String("flaggedBy", internalUserID),
		zap.String("reason", reason),
	)
	return s.messageRepo.Flag(ctx, messageID, internalUserID)
}

// GetFlaggedMessageContent déchiffre et retourne le contenu d'un message signalé.
func (s *chatServiceImpl) GetFlaggedMessageContent(ctx context.Context, messageID string) (string, *domain.ChatMessage, error) {
	msg, err := s.messageRepo.GetByID(ctx, messageID)
	if err != nil {
		return "", nil, fmt.Errorf("chat: get message: %w", err)
	}
	if msg == nil {
		return "", nil, chatErrors.ErrMessageNotFound
	}
	if !msg.Flagged {
		return "", nil, chatErrors.ErrMessageNotFlagged
	}
	plaintext, err := s.encryptor.Decrypt(msg.ContentEncrypted, msg.ContentNonce, msg.ThreadID)
	if err != nil {
		return "", nil, fmt.Errorf("chat: decrypt flagged message: %w", err)
	}
	return string(plaintext), msg, nil
}

// CloseThreadsByTripID ferme tous les threads actifs d'un trajet terminé.
func (s *chatServiceImpl) CloseThreadsByTripID(ctx context.Context, tripID string) (int, error) {
	n, err := s.threadRepo.CloseByTripID(ctx, tripID)
	if err != nil {
		return 0, fmt.Errorf("chat: close threads for trip %s: %w", tripID, err)
	}
	if n > 0 {
		s.logger.Info("chat: threads closed on trip completion",
			zap.String("tripID", tripID),
			zap.Int("count", n),
		)
	}
	return n, nil
}

// resolveRole retourne le rôle de userID dans le thread.
func (s *chatServiceImpl) resolveRole(thread *domain.ChatThread, userID string) (domain.SenderRole, error) {
	switch userID {
	case thread.DriverID:
		return domain.SenderRoleDriver, nil
	case thread.PassengerID:
		return domain.SenderRolePassenger, nil
	default:
		return "", chatErrors.ErrUnauthorized
	}
}
