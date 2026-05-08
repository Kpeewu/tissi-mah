package grpc

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatpb "github.com/Kpeewu/tissi-mah/services/chat-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/middleware"
	svcInterfaces "github.com/Kpeewu/tissi-mah/services/chat-service/internal/service/interfaces"
	chatErrors "github.com/Kpeewu/tissi-mah/services/chat-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/crypto"
)

// ChatHandler implémente le serveur gRPC ChatService.
type ChatHandler struct {
	chatpb.UnimplementedChatServiceServer
	service   svcInterfaces.ChatService
	encryptor *crypto.MessageEncryptor
	logger    *zap.Logger
}

func NewChatHandler(service svcInterfaces.ChatService, encryptor *crypto.MessageEncryptor, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{service: service, encryptor: encryptor, logger: logger}
}

func (h *ChatHandler) Health(ctx context.Context, _ *chatpb.HealthRequest) (*chatpb.HealthResponse, error) {
	return &chatpb.HealthResponse{Status: "ok"}, nil
}

// =============================================================================
// GetOrCreateThread
// =============================================================================

func (h *ChatHandler) GetOrCreateThread(ctx context.Context, req *chatpb.GetOrCreateThreadRequest) (*chatpb.GetOrCreateThreadResponse, error) {
	userID := getUserID(ctx)
	thread, err := h.service.GetOrCreateThread(ctx, svcInterfaces.GetOrCreateThreadInput{
		UserID:    userID,
		BookingID: req.BookingId,
	})
	if err != nil {
		return &chatpb.GetOrCreateThreadResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}
	return &chatpb.GetOrCreateThreadResponse{Thread: toProtoThread(thread, 0, nil)}, nil
}

// =============================================================================
// SendMessage
// =============================================================================

func (h *ChatHandler) SendMessage(ctx context.Context, req *chatpb.SendMessageRequest) (*chatpb.SendMessageResponse, error) {
	userID := getUserID(ctx)
	result, err := h.service.SendMessage(ctx, svcInterfaces.SendMessageInput{
		UserID:   userID,
		ThreadID: req.ThreadId,
		Content:  req.Content,
	})
	if err != nil {
		return &chatpb.SendMessageResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}
	// Déchiffrer pour la réponse immédiate (l'expéditeur voit son propre message).
	plaintext, decErr := h.encryptor.Decrypt(result.Message.ContentEncrypted, result.Message.ContentNonce, result.Message.ThreadID)
	if decErr != nil {
		h.logger.Error("chat: decrypt for send response", zap.Error(decErr))
		plaintext = []byte("[contenu chiffré]")
	}
	return &chatpb.SendMessageResponse{
		Message:      toProtoMessage(result.Message, string(plaintext)),
		HasRedaction: result.HasRedaction,
	}, nil
}

// =============================================================================
// GetMessages
// =============================================================================

func (h *ChatHandler) GetMessages(ctx context.Context, req *chatpb.GetMessagesRequest) (*chatpb.GetMessagesResponse, error) {
	userID := getUserID(ctx)
	result, err := h.service.GetMessages(ctx, svcInterfaces.GetMessagesInput{
		UserID:         userID,
		ThreadID:       req.ThreadId,
		AfterMessageID: req.AfterMessageId,
		Limit:          int(req.PageSize),
	})
	if err != nil {
		return &chatpb.GetMessagesResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	protoMsgs := make([]*chatpb.MessageInfo, 0, len(result.Messages))
	for _, m := range result.Messages {
		plaintext, decErr := h.encryptor.Decrypt(m.ContentEncrypted, m.ContentNonce, m.ThreadID)
		if decErr != nil {
			h.logger.Error("chat: decrypt message", zap.String("messageID", m.MessageID), zap.Error(decErr))
			plaintext = []byte("[contenu indisponible]")
		}
		protoMsgs = append(protoMsgs, toProtoMessage(m, string(plaintext)))
	}

	return &chatpb.GetMessagesResponse{
		Messages:    protoMsgs,
		HasMore:     result.HasMore,
		UnreadCount: result.UnreadCount,
	}, nil
}

// =============================================================================
// GetUserThreads
// =============================================================================

func (h *ChatHandler) GetUserThreads(ctx context.Context, req *chatpb.GetUserThreadsRequest) (*chatpb.GetUserThreadsResponse, error) {
	userID := getUserID(ctx)
	threads, err := h.service.GetUserThreads(ctx, svcInterfaces.GetUserThreadsInput{
		UserID:        userID,
		Limit:         int(req.PageSize),
		AfterThreadID: req.PageToken,
	})
	if err != nil {
		return &chatpb.GetUserThreadsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	protoThreads := make([]*chatpb.ThreadInfo, 0, len(threads))
	for _, t := range threads {
		// Charger le dernier message pour le preview
		var lastMsgProto *chatpb.MessageInfo
		// (enrichissement optionnel — omis pour éviter N+1; le client peut
		// charger GetMessages séparément si besoin)
		protoThreads = append(protoThreads, toProtoThread(t, 0, lastMsgProto))
	}

	nextToken := ""
	if len(threads) > 0 {
		nextToken = threads[len(threads)-1].ThreadID
	}

	return &chatpb.GetUserThreadsResponse{
		Threads:       protoThreads,
		NextPageToken: nextToken,
	}, nil
}

// =============================================================================
// MarkRead
// =============================================================================

func (h *ChatHandler) MarkRead(ctx context.Context, req *chatpb.MarkReadRequest) (*chatpb.MarkReadResponse, error) {
	userID := getUserID(ctx)
	if err := h.service.MarkRead(ctx, userID, req.ThreadId, req.LastReadMessageId); err != nil {
		return &chatpb.MarkReadResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}
	return &chatpb.MarkReadResponse{}, nil
}

// =============================================================================
// FlagMessage
// =============================================================================

func (h *ChatHandler) FlagMessage(ctx context.Context, req *chatpb.FlagMessageRequest) (*chatpb.FlagMessageResponse, error) {
	userID := getUserID(ctx)
	if err := h.service.FlagMessage(ctx, userID, req.MessageId, req.Reason); err != nil {
		return &chatpb.FlagMessageResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}
	return &chatpb.FlagMessageResponse{}, nil
}

// =============================================================================
// GetFlaggedMessageContent (support uniquement)
// =============================================================================

func (h *ChatHandler) GetFlaggedMessageContent(ctx context.Context, req *chatpb.GetFlaggedMessageContentRequest) (*chatpb.GetFlaggedMessageContentResponse, error) {
	plaintext, msg, err := h.service.GetFlaggedMessageContent(ctx, req.MessageId)
	if err != nil {
		return &chatpb.GetFlaggedMessageContentResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}
	resp := &chatpb.GetFlaggedMessageContentResponse{
		PlaintextContent: plaintext,
		SenderId:         msg.SenderID,
		SenderRole:       string(msg.SenderRole),
		CreatedAt:        msg.CreatedAt.Format(time.RFC3339),
	}
	if msg.FlaggedBy != nil {
		resp.FlaggedBy = *msg.FlaggedBy
	}
	if msg.FlaggedAt != nil {
		resp.FlaggedAt = msg.FlaggedAt.Format(time.RFC3339)
	}
	return resp, nil
}

// =============================================================================
// CloseThread (interne / support)
// =============================================================================

func (h *ChatHandler) CloseThread(ctx context.Context, req *chatpb.CloseThreadRequest) (*chatpb.CloseThreadResponse, error) {
	tripID := req.TripId
	if tripID == "" && req.ThreadId != "" {
		// Fermeture par threadID : on passe tripID vide, le service gérera par threadID
		// (non implémenté ici pour garder simple — on utilise CloseByTripID)
		return &chatpb.CloseThreadResponse{ErrorMessage: "use TripId to close threads"}, nil
	}
	n, err := h.service.CloseThreadsByTripID(ctx, tripID)
	if err != nil {
		return &chatpb.CloseThreadResponse{ErrorMessage: err.Error()}, status.Error(codes.Internal, err.Error())
	}
	return &chatpb.CloseThreadResponse{ThreadsClosed: int32(n)}, nil
}

// =============================================================================
// Helpers
// =============================================================================

func getUserID(ctx context.Context) string {
	uid, _ := ctx.Value(middleware.FirebaseIDKey).(string)
	return uid
}

func toProtoMessage(m *domain.ChatMessage, plaintext string) *chatpb.MessageInfo {
	proto := &chatpb.MessageInfo{
		MessageId:    m.MessageID,
		ThreadId:     m.ThreadID,
		SenderId:     m.SenderID,
		SenderRole:   string(m.SenderRole),
		Content:      plaintext,
		HasRedaction: m.HasRedaction,
		Flagged:      m.Flagged,
		CreatedAt:    m.CreatedAt.Format(time.RFC3339),
	}
	if m.ReadAt != nil {
		proto.ReadAt = m.ReadAt.Format(time.RFC3339)
	}
	return proto
}

func toProtoThread(t *domain.ChatThread, unreadCount int32, lastMsg *chatpb.MessageInfo) *chatpb.ThreadInfo {
	proto := &chatpb.ThreadInfo{
		ThreadId:    t.ThreadID,
		BookingId:   t.BookingID,
		TripId:      t.TripID,
		DriverId:    t.DriverID,
		PassengerId: t.PassengerID,
		Status:      string(t.Status),
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		UnreadCount: unreadCount,
		LastMessage: lastMsg,
	}
	if t.ClosedAt != nil {
		proto.ClosedAt = t.ClosedAt.Format(time.RFC3339)
	}
	return proto
}

func toGRPCError(err error) error {
	switch err {
	case chatErrors.ErrUnauthorized:
		return status.Error(codes.PermissionDenied, err.Error())
	case chatErrors.ErrBookingNotFound, chatErrors.ErrThreadNotFound, chatErrors.ErrMessageNotFound:
		return status.Error(codes.NotFound, err.Error())
	case chatErrors.ErrBookingNotApproved, chatErrors.ErrTripAlreadyEnded, chatErrors.ErrThreadClosed:
		return status.Error(codes.FailedPrecondition, err.Error())
	case chatErrors.ErrEmptyMessage, chatErrors.ErrMessageTooLong,
		chatErrors.ErrMissingBookingID, chatErrors.ErrMissingThreadID, chatErrors.ErrMissingMessageID:
		return status.Error(codes.InvalidArgument, err.Error())
	case chatErrors.ErrBookingServiceUnavailable, chatErrors.ErrTripServiceUnavailable:
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
