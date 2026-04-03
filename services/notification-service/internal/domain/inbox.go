package domain

import "time"

// InboxEntry représente une notification visible dans l'inbox de l'app mobile.
type InboxEntry struct {
	InboxID        string
	UserID         string
	EventType      string
	Title          string
	Body           string
	ActionType     string // "booking_detail", "trip_detail", etc.
	ActionID       string
	IsRead         bool
	ReadAt         *time.Time
	NotificationID string
	CreatedAt      time.Time
}
