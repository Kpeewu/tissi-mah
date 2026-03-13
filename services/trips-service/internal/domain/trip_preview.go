package domain

import "time"

// TripPreview contient les données brutes d'un trajet pour la liste du conducteur.
// Les champs DriverName, VehicleBrand, VehiclePlate sont enrichis par le service.
type TripPreview struct {
	TripID                string
	DriverID              string
	VehicleID             string
	DepartureDatetime     time.Time
	TotalSeats            int16
	AvailableSeats        int16
	DepartureLocationName string
	ArrivalLocationName   string
}
