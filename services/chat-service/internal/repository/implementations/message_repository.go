package implementations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/repository/interfaces"
)

type chatMessageRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewChatMessageRepository(db *pgxpool.Pool, logger *zap.Logger) interfaces.ChatMessageRepository {
	return &chatMessageRepository{db: db, logger: logger}
}

func (r *chatMessageRepository) Create(ctx context.Context, m *domain.ChatMessage) (*domain.ChatMessage, error) {
	const q = `
		INSERT INTO chat_messages
		  (message_id, thread_id, sender_id, sender_role, content_encrypted, content_nonce, has_redaction)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING message_id, thread_id, sender_id, sender_role,
		          content_encrypted, content_nonce, has_redaction,
		          flagged, flagged_by, flagged_at, created_at, read_at`
	row := r.db.QueryRow(ctx, q,
		m.MessageID, m.ThreadID, m.SenderID, string(m.SenderRole),
		m.ContentEncrypted, m.ContentNonce, m.HasRedaction,
	)
	return scanMessage(row)
}

func (r *chatMessageRepository) GetByID(ctx context.Context, messageID string) (*domain.ChatMessage, error) {
	const q = `SELECT message_id, thread_id, sender_id, sender_role,
	                  content_encrypted, content_nonce, has_redaction,
	                  flagged, flagged_by, flagged_at, created_at, read_at
	           FROM chat_messages WHERE message_id = $1`
	return scanMessage(r.db.QueryRow(ctx, q, messageID))
}

func (r *chatMessageRepository) GetByThreadID(ctx context.Context, threadID string, afterMessageID string, limit int) ([]*domain.ChatMessage, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if afterMessageID == "" {
		// Retourner les N derniers messages (ordre chronologique)
		const q = `
			SELECT message_id, thread_id, sender_id, sender_role,
			       content_encrypted, content_nonce, has_redaction,
			       flagged, flagged_by, flagged_at, created_at, read_at
			FROM chat_messages
			WHERE thread_id = $1
			ORDER BY created_at DESC
			LIMIT $2`
		rows, err = r.db.Query(ctx, q, threadID, limit)
	} else {
		// Pagination forward depuis un curseur
		const q = `
			SELECT message_id, thread_id, sender_id, sender_role,
			       content_encrypted, content_nonce, has_redaction,
			       flagged, flagged_by, flagged_at, created_at, read_at
			FROM chat_messages
			WHERE thread_id = $1
			  AND created_at > (SELECT created_at FROM chat_messages WHERE message_id = $2)
			ORDER BY created_at ASC
			LIMIT $3`
		rows, err = r.db.Query(ctx, q, threadID, afterMessageID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectMessages(rows)
}

func (r *chatMessageRepository) GetLastMessage(ctx context.Context, threadID string) (*domain.ChatMessage, error) {
	const q = `SELECT message_id, thread_id, sender_id, sender_role,
	                  content_encrypted, content_nonce, has_redaction,
	                  flagged, flagged_by, flagged_at, created_at, read_at
	           FROM chat_messages
	           WHERE thread_id = $1
	           ORDER BY created_at DESC LIMIT 1`
	return scanMessage(r.db.QueryRow(ctx, q, threadID))
}

func (r *chatMessageRepository) CountUnread(ctx context.Context, threadID string, userID string) (int32, error) {
	// Messages envoyés par l'autre participant et non encore lus par userID.
	const q = `
		SELECT COUNT(*) FROM chat_messages
		WHERE thread_id = $1 AND sender_id != $2 AND read_at IS NULL`
	var count int32
	err := r.db.QueryRow(ctx, q, threadID, userID).Scan(&count)
	return count, err
}

func (r *chatMessageRepository) MarkReadUntil(ctx context.Context, threadID string, userID string, messageID string) error {
	const q = `
		UPDATE chat_messages
		SET read_at = NOW()
		WHERE thread_id = $1
		  AND sender_id != $2
		  AND read_at IS NULL
		  AND created_at <= (SELECT created_at FROM chat_messages WHERE message_id = $3)`
	_, err := r.db.Exec(ctx, q, threadID, userID, messageID)
	return err
}

func (r *chatMessageRepository) Flag(ctx context.Context, messageID string, flaggedBy string) error {
	now := time.Now().UTC()
	const q = `
		UPDATE chat_messages
		SET flagged = TRUE, flagged_by = $2, flagged_at = $3
		WHERE message_id = $1 AND flagged = FALSE`
	_, err := r.db.Exec(ctx, q, messageID, flaggedBy, now)
	return err
}

func (r *chatMessageRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	const q = `
		DELETE FROM chat_messages
		WHERE created_at < $1 AND flagged = FALSE`
	tag, err := r.db.Exec(ctx, q, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func scanMessage(row pgx.Row) (*domain.ChatMessage, error) {
	var m domain.ChatMessage
	var role string
	err := row.Scan(
		&m.MessageID, &m.ThreadID, &m.SenderID, &role,
		&m.ContentEncrypted, &m.ContentNonce, &m.HasRedaction,
		&m.Flagged, &m.FlaggedBy, &m.FlaggedAt, &m.CreatedAt, &m.ReadAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	m.SenderRole = domain.SenderRole(role)
	return &m, nil
}

func collectMessages(rows pgx.Rows) ([]*domain.ChatMessage, error) {
	var msgs []*domain.ChatMessage
	for rows.Next() {
		var m domain.ChatMessage
		var role string
		if err := rows.Scan(
			&m.MessageID, &m.ThreadID, &m.SenderID, &role,
			&m.ContentEncrypted, &m.ContentNonce, &m.HasRedaction,
			&m.Flagged, &m.FlaggedBy, &m.FlaggedAt, &m.CreatedAt, &m.ReadAt,
		); err != nil {
			return nil, err
		}
		m.SenderRole = domain.SenderRole(role)
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

// AnonymizeUserRefs pseudonymise sender_id dans les messages de l'utilisateur.
func (r *chatMessageRepository) AnonymizeUserRefs(ctx context.Context, userID string) error {
	anon := "deleted_" + userID[:8]
	_, err := r.db.Exec(ctx,
		`UPDATE chat_messages SET sender_id = $1 WHERE sender_id = $2`,
		anon, userID,
	)
	if err != nil {
		r.logger.Error("AnonymizeUserRefs: update message sender_id failed", zap.Error(err), zap.String("userID", userID))
		return err
	}
	return nil
}
