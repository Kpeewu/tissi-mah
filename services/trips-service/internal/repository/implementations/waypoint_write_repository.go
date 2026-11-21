package implementations

// waypointWriteRepositoryImpl est réservé aux opérations futures sur les waypoints
// hors création de trajet (ex: mise à jour du pickup réel).
// La création atomique trip + waypoints est gérée par TripWriteRepository.Create.
type waypointWriteRepositoryImpl struct{}
