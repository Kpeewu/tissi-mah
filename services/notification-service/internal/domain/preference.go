package domain

import "time"

// UserNotificationPreference stocke les préférences de notification d'un utilisateur.
type UserNotificationPreference struct {
	PreferenceID string
	UserID       string
	PushEnabled  bool
	EmailEnabled bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
