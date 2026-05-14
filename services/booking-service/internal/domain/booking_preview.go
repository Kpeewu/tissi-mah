package domain

import "time"

// BookingPreview contient les informations résumées d'une réservation pour l'affichage en liste.
type BookingPreview struct {
	BookingID           string
	BookingReference    string
	TripID              string
	Status              BookingStatus
	SeatsBooked         int16
	TotalAmount         int
	PickupLocationName  string
	DropoffLocationName string
	DepartureDatetime   time.Time
}

// RawDriverBookingPreview contient les champs DB bruts avant enrichissement externe.
// Utilisé par GetDriverPendingBookings et GetDriverTripBookingsRaw.
type RawDriverBookingPreview struct {
	BookingID           string
	BookingReference    string
	TripID              string
	PassengerID         string
	Status              BookingStatus
	SeatsBooked         int16
	TotalAmount         int
	PickupLocationName  string
	DropoffLocationName string
	DepartureDatetime   time.Time // depuis bookings_segments.pickup_scheduled_at (COALESCE created_at)
	PaymentMethod       string
	PassengerMessage    *string
	ExtraMinutesDetour  *int16
	CreatedAt           time.Time
	PaymentCompletedAt  *time.Time
}

// BookingCounts regroupe les compteurs de réservations par statut pour un trajet.
type BookingCounts struct {
	Pending   int32
	Approved  int32
	Rejected  int32
	Cancelled int32
}

// RawPassengerSummary contient les champs DB bruts pour GetActivePassengerSummariesForTrip.
type RawPassengerSummary struct {
	PassengerID        string
	BookingID          string
	SeatsBooked        int16
	PaymentMethod      string
	PaymentCompletedAt *time.Time
}
