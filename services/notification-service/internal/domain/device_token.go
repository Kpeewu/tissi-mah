package domain

import "time"

// UserDeviceToken représente un token FCM enregistré pour un utilisateur.
type UserDeviceToken struct {
	TokenID    string
	UserID     string
	FCMToken   string
	Platform   string // "android", "ios", "web"
	DeviceName string
	IsActive   bool
	LastUsedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
