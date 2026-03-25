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
