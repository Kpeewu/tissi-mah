// Package errors centralise les sentinel errors du geolocation-service.
package errors

import "errors"

// Erreurs de validation d'input.
var (
	// ErrorInvalidWaypoints — moins de 2 waypoints, ou coordonnées hors plage.
	ErrorInvalidWaypoints = errors.New("ErrorInvalidWaypoints")

	// ErrorInvalidProfile — profile non supporté (V1 : "driving" uniquement).
	ErrorInvalidProfile = errors.New("ErrorInvalidProfile")

	// ErrorInvalidDepartureTime — DepartureTime fourni mais pas du RFC3339 valide.
	ErrorInvalidDepartureTime = errors.New("ErrorInvalidDepartureTime")

	// ErrorEmptyQuery — Geocode appelé avec Query vide.
	ErrorEmptyQuery = errors.New("ErrorEmptyQuery")
)

// Erreurs de routage / geocoding.
var (
	// ErrorRouteNotFound — OSRM n'a pas trouvé de route entre les waypoints.
	ErrorRouteNotFound = errors.New("ErrorRouteNotFound")

	// ErrorAddressNotFound — Nominatim n'a renvoyé aucun résultat pour la requête.
	ErrorAddressNotFound = errors.New("ErrorAddressNotFound")
)

// Erreurs de protection runtime (semaphore, circuit breaker, timeouts).
var (
	// ErrorRoutingOverloaded — semaphore plein, le service refuse la requête plutôt
	// que d'overloader OSRM. Le client peut retry après backoff.
	ErrorRoutingOverloaded = errors.New("ErrorRoutingOverloaded")

	// ErrorRoutingUnavailable — circuit breaker ouvert OU OSRM injoignable.
	// Indique au client que le routing est temporairement KO.
	ErrorRoutingUnavailable = errors.New("ErrorRoutingUnavailable")

	// ErrorGeocodingOverloaded — pendant Nominatim.
	ErrorGeocodingOverloaded = errors.New("ErrorGeocodingOverloaded")

	// ErrorGeocodingUnavailable — pendant Nominatim.
	ErrorGeocodingUnavailable = errors.New("ErrorGeocodingUnavailable")
)

// Erreurs internes.
var (
	// ErrorInternalServer — erreur non catégorisée (à éviter).
	ErrorInternalServer = errors.New("ErrorInternalServer")
)
