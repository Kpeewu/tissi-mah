package domain

import "time"

// EventRouting définit la configuration de routage pour un type d'événement.
type EventRouting struct {
	RoutingID        string
	EventType        string
	SendPush         bool
	SendEmail        bool
	CreateInboxEntry bool
	Priority         string // "critical", "standard", "low"
	IsActive         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// MaxAttemptsForPriority retourne le nombre max de tentatives selon la priorité.
func MaxAttemptsForPriority(priority string) int16 {
	switch priority {
	case "critical":
		return 5
	case "standard":
		return 3
	default:
		return 1
	}
}
