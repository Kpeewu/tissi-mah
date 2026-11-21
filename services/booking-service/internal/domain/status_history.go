package domain

import "time"

// StatusHistoryEntry représente une entrée dans l'historique des changements de statut.
type StatusHistoryEntry struct {
	HistoryID     string
	BookingID     string
	PreviousStatus string
	NewStatus      string
	ChangedBy      string
	ChangedByType  string // passenger | driver | admin | system
	ChangeReason   *string
	Metadata       *string // JSON string
	CreatedAt      time.Time
}
