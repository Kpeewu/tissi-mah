package gateway

// ProtectedRoutes liste les routes HTTP qui requièrent un JWT Firebase valide.
// Les routes absentes de cette map sont publiques (Health, CheckEmail, CheckPhoneNumber).
// Les clés correspondent aux paths définis dans les annotations google.api.http des protos.
var ProtectedRoutes = map[string]bool{
	// auth-service
	"/api/v1/auth/login":         true,
	"/api/v1/auth/createAccount": true,
	"/api/v1/auth/deleteAccount": true,

	// user-service
	"/api/v1/user/me":                        true,
	"/api/v1/userProfile/createDriverAccount": true,
	"/api/v1/userProfile/addTripPreferences":  true,
	"/api/v1/userProfile/updateProfile":       true,

	// rating-service — tous les endpoints publics (rater_id dans le body)
	// TODO: re-enable JWT protection once Firebase JWT is fully integrated

	// file-service
	"/file/uploadIdDocument":       true,
	"/file/uploadVehicleDocuments": true,
	"/file/changeDocument":         true,
	"/file/getDocument":            true,
	"/file/deleteFile":             true,

	// vehicle-service
	"/vehicle/add":             true,
	"/vehicle/update":          true,
	"/vehicle/delete":          true,
	"/vehicle/details":         true,
	"/vehicle/getUserVehicles": true,

	// trips-service
	"/trip/driver/createTrip":                true,
	"/trip/driver/createRecurringTrip":        true,
	"/trip/driver/getTripsPreviews":           true,
	"/trip/driver/getCompletedTripsPreviews":  true,
	"/trip/driver/changeTripDateAndTime": true,
	"/trip/driver/changeTripVehicle":     true,
	"/trip/driver/changeTripAllowances":  true,
	"/trip/driver/activeAutoApprouve":    true,
	"/trip/driver/startTrip":                  true,
	"/trip/driver/endTrip":                    true,
	"/trip/driver/confirmWaypointArrival":     true,
	"/trip/driver/confirmWaypointDeparture":   true,
}

// RateLimitTier identifie le niveau de rate limiting pour une route
type RateLimitTier string

const (
	TierGlobal        RateLimitTier = "global"
	TierAuth          RateLimitTier = "auth"
	TierCreateAccount RateLimitTier = "create"
	TierSensitive     RateLimitTier = "sensitive"
)

// RouteRateLimitConfig associe chaque route à son tier de rate limiting.
// Les routes absentes utilisent le tier global par défaut.
var RouteRateLimitConfig = map[string]RateLimitTier{
	// Auth endpoints — brute force protection
	"/api/v1/auth/login":            TierAuth,
	"/api/v1/auth/checkPhoneNumber": TierAuth,
	"/api/v1/auth/checkEmail":       TierAuth,

	// Création de compte — anti-spam
	"/api/v1/auth/createAccount": TierCreateAccount,

	// Opérations sensibles
	"/api/v1/auth/deleteAccount": TierSensitive,
}
