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

type chatThreadRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewChatThreadRepository(db *pgxpool.Pool, logger *zap.Logger) interfaces.ChatThreadRepository {
	return &chatThreadRepository{db: db, logger: logger}
}

func (r *chatThreadRepository) Create(ctx context.Context, t *domain.ChatThread) (*domain.ChatThread, error) {
	const q = `
		INSERT INTO chat_threads (thread_id, booking_id, trip_id, driver_id, passenger_id, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING thread_id, booking_id, trip_id, driver_id, passenger_id, status,
		          created_at, closed_at, updated_at`
	row := r.db.QueryRow(ctx, q,
		t.ThreadID, t.BookingID, t.TripID, t.DriverID, t.PassengerID, string(t.Status))
	return scanThread(row)
}

func (r *chatThreadRepository) GetByID(ctx context.Context, threadID string) (*domain.ChatThread, error) {
	const q = `SELECT thread_id, booking_id, trip_id, driver_id, passenger_id, status,
	                  created_at, closed_at, updated_at
	           FROM chat_threads WHERE thread_id = $1`
	return scanThread(r.db.QueryRow(ctx, q, threadID))
}

func (r *chatThreadRepository) GetByBookingID(ctx context.Context, bookingID string) (*domain.ChatThread, error) {
	const q = `SELECT thread_id, booking_id, trip_id, driver_id, passenger_id, status,
	                  created_at, closed_at, updated_at
	           FROM chat_threads WHERE booking_id = $1`
	return scanThread(r.db.QueryRow(ctx, q, bookingID))
}

func (r *chatThreadRepository) GetByUserID(ctx context.Context, userID string, limit int, afterThreadID string) ([]*domain.ChatThread, error) {
	const q = `
		SELECT thread_id, booking_id, trip_id, driver_id, passenger_id, status,
		       created_at, closed_at, updated_at
		FROM chat_threads
		WHERE (driver_id = $1 OR passenger_id = $1)
		  AND ($2::text = '' OR updated_at < (SELECT updated_at FROM chat_threads WHERE thread_id = $2::uuid))
		ORDER BY updated_at DESC
		LIMIT $3`
	rows, err := r.db.Query(ctx, q, userID, afterThreadID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectThreads(rows)
}

func (r *chatThreadRepository) UpdateStatus(ctx context.Context, threadID string, status domain.ThreadStatus, closedAt *time.Time) error {
	const q = `UPDATE chat_threads SET status = $2, closed_at = $3, updated_at = NOW() WHERE thread_id = $1`
	_, err := r.db.Exec(ctx, q, threadID, string(status), closedAt)
	return err
}

func (r *chatThreadRepository) CloseByTripID(ctx context.Context, tripID string) (int, error) {
	now := time.Now().UTC()
	const q = `UPDATE chat_threads
	           SET status = 'closed', closed_at = $2, updated_at = NOW()
	           WHERE trip_id = $1 AND status = 'active'`
	tag, err := r.db.Exec(ctx, q, tripID, now)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func scanThread(row pgx.Row) (*domain.ChatThread, error) {
	var t domain.ChatThread
	var status string
	err := row.Scan(
		&t.ThreadID, &t.BookingID, &t.TripID, &t.DriverID, &t.PassengerID,
		&status, &t.CreatedAt, &t.ClosedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	t.Status = domain.ThreadStatus(status)
	return &t, nil
}

func collectThreads(rows pgx.Rows) ([]*domain.ChatThread, error) {
	var threads []*domain.ChatThread
	for rows.Next() {
		var t domain.ChatThread
		var status string
		if err := rows.Scan(
			&t.ThreadID, &t.BookingID, &t.TripID, &t.DriverID, &t.PassengerID,
			&status, &t.CreatedAt, &t.ClosedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		t.Status = domain.ThreadStatus(status)
		threads = append(threads, &t)
	}
	return threads, rows.Err()
}
