package osrm

// osrmResponse correspond à la réponse JSON de l'API /route/v1 d'OSRM.
// On ne décode que les champs utilisés par geolocation-service.
type osrmResponse struct {
	Code    string      `json:"code"`              // "Ok" en cas de succès
	Message string      `json:"message,omitempty"` // détail si Code != "Ok"
	Routes  []osrmRoute `json:"routes,omitempty"`
}

type osrmRoute struct {
	Distance float64   `json:"distance"` // mètres
	Duration float64   `json:"duration"` // secondes
	Geometry string    `json:"geometry"` // Google polyline encodée (profile geometries=polyline)
	Legs     []osrmLeg `json:"legs"`
}

type osrmLeg struct {
	Distance float64 `json:"distance"` // mètres pour ce leg
	Duration float64 `json:"duration"` // secondes pour ce leg
	Summary  string  `json:"summary,omitempty"`
}

// RouteResult est le contrat retourné par le client OSRM (mapping intermédiaire
// avant la conversion vers serviceInterfaces.ComputeRouteResult par le service).
type RouteResult struct {
	DistanceMeters  float64
	DurationSeconds float64
	Polyline        string
	Legs            []LegResult
}

type LegResult struct {
	DistanceMeters  float64
	DurationSeconds float64
}
