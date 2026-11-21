package interfaces

// WaypointRepositoryWrite est réservé aux opérations de waypoints hors création de trajet.
// Les waypoints à la création sont gérés atomiquement par TripRepositoryWrite.Create.
type WaypointRepositoryWrite interface{}
