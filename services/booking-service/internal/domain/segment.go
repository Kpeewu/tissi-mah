package domain

import "time"

// Segment représente un segment d'une réservation.
type Segment struct {
	SegmentID              string
	BookingID              string
	PickupWaypointID       string
	DropoffWaypointID      string
	PickupLocationName     string
	PickupCity             string
	PickupLat              float64
	PickupLng              float64
	PickupScheduledAt      *time.Time
	PickupActualAt         *time.Time
	DropoffLocationName    string
	DropoffCity            string
	DropoffLat             float64
	DropoffLng             float64
	DropoffScheduledAt     *time.Time
	DropoffActualAt        *time.Time
	SegmentDistanceMeters  *int
	SegmentDurationMinutes *int
	SegmentPrice           *int
	CreatedAt              time.Time
}
