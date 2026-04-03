package domain

import "time"

// Notification représente une entrée dans le journal technique d'envoi.
type Notification struct {
	NotificationID    string
	EventID           string
	UserID            string
	EventType         string
	Channel           string // "push", "email"
	TemplateID        string
	ResolvedTitle     string
	ResolvedSubject   string
	ResolvedBody      string
	RecipientAddress  string
	Status            string // "pending", "processing", "sent", "delivered", "failed", "cancelled"
	FailureReason     string
	ReferenceID       string
	ReferenceType     string
	AttemptCount      int16
	MaxAttempts       int16
	NextAttemptAt     *time.Time
	LastAttemptAt     *time.Time
	ProviderName      string // "fcm", "sendgrid", "aws_ses"
	ProviderMessageID string
	SentAt            *time.Time
	DeliveredAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
