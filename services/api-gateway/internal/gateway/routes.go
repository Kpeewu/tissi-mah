package gateway

// ProtectedRoutes liste les routes HTTP qui requièrent un JWT Firebase valide.
// Les routes absentes de cette map sont publiques (Health, CheckEmail, CheckPhoneNumber).
// Les clés correspondent aux paths définis dans les annotations google.api.http des protos.
var ProtectedRoutes = map[string]bool{
	// auth-service
	"/api/v1/auth/createAccount": true,
	"/api/v1/auth/deleteAccount": true,

	// user-service
	"/api/v1/user/me":                         true,
	"/api/v1/userProfile/createDriverAccount":  true,
	"/api/v1/userProfile/addTripPreferences":   true,
	"/api/v1/userProfile/updateProfile":        true,
	"/api/v1/userProfile/changeProfilePicture": true,

	// rating-service
	"/api/v1/ratings/rateUser":                    true,
	"/api/v1/ratings/user/getUserRatings":         true,
	"/api/v1/ratings/user/getUserRatingsAverage":  true,
	"/api/v1/ratings/updateRating":                true,

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

	// kyc-service (user-facing)
	"/api/v1/kyc/inquiries/add":        true,
	"/api/v1/kyc/inquiries/getInquiry": true,
	"/api/v1/kyc/me/getStatus":         true,
	"/api/v1/kyc/inquiries/resume":     true,
	// kyc-service (admin)
	"/api/v1/kyc/admin/reviews/getReviews": true,
	"/api/v1/kyc/admin/reviews/getReview":  true,
	"/api/v1/kyc/admin/reviews/override":   true,
	// kyc-service — webhook et health sont publics (pas de JWT)

	// booking-service (internal/* et health sont publics)
	"/booking/createBooking":        true,
	"/booking/getBookingDetails":    true,
	"/booking/getPassengerBookings": true,
	"/booking/getDriverTripBookings": true,
	"/booking/approveBooking":       true,
	"/booking/rejectBooking":        true,
	"/booking/cancelBooking":        true,
	"/booking/reportNoShow":         true,
	"/booking/confirmPayment":       true,

	// payment-service (protégé)
	"/payment/createPayment":       true,
	"/payment/getPaymentStatus":    true,
	"/payment/getPaymentByBooking": true,
	"/payment/getRefundStatus":     true,
	"/payment/getPayoutStatus":     true,
	"/payment/getDriverPayouts":    true,
	// payment-service — webhooks, internal et health sont publics

	// trips-service
	"/trip/driver/createTrip":                true,
	"/trip/driver/createRecurringTrip":       true,
	"/trip/driver/getTripsPreviews":          true,
	"/trip/driver/getCompletedTripsPreviews": true,
	"/trip/driver/changeTripDateAndTime":     true,
	"/trip/driver/changeTripVehicle":         true,
	"/trip/driver/changeTripAllowances":      true,
	"/trip/driver/activeAutoApprouve":        true,
	"/trip/driver/startTrip":                 true,
	"/trip/driver/endTrip":                   true,
	"/trip/driver/confirmWaypointArrival":    true,
	"/trip/driver/confirmWaypointDeparture":  true,
	"/trip/driver/cancelWaypoint":            true,
	"/trip/driver/cancelTrip":               true,
	"/trip/driver/getTripDetails":            true,

	// notification-service
	"/api/v1/notifications/inbox":                 true,
	"/api/v1/notifications/inbox/{inbox_id}/read": true,
	"/api/v1/notifications/inbox/readAll":         true,
	"/api/v1/notifications/inbox/unreadCount":     true,
	"/api/v1/notifications/preferences":           true,
	"/api/v1/notifications/deviceToken":            true,
	// notification-service — health est public (pas de JWT)

	// geolocation-service (Firebase JWT requis pendant la création de trajet)
	"/api/v1/geolocation/route":   true,
	"/api/v1/geolocation/geocode": true,
	"/api/v1/geolocation/reverse": true,
	// geolocation-service — health est public (pas de JWT)

	// chat-service (passager-chauffeur, après réservation acceptée)
	"/api/v1/chat/threads":                                true, // POST GetOrCreateThread + GET GetUserThreads
	"/api/v1/chat/threads/{thread_id}/messages":           true, // POST SendMessage + GET GetMessages
	"/api/v1/chat/threads/{thread_id}/read":               true, // PATCH MarkRead
	"/api/v1/chat/messages/{message_id}/flag":             true, // POST FlagMessage
	// chat-service — health public ; GetFlaggedMessageContent côté Support (cf. SupportProtectedRoutes)
}

// SupportProtectedRoutes liste les routes HTTP qui requièrent un JWT support-service valide
// (back-office admin / agents support — distinct du JWT Firebase utilisé pour les passagers/conducteurs).
var SupportProtectedRoutes = map[string]bool{
	"/api/v1/support/logout":                  true,
	"/api/v1/support/me":                      true,
	"/api/v1/support/me/password":             true,
	"/api/v1/support/me/email":                true,
	"/api/v1/support/admin/agents":            true,
	"/api/v1/support/admin/agents/deactivate": true,

	// payment-service — actions support
	"/payment/support/triggerManualPayout": true,

	// chat-service — accès support au contenu déchiffré d'un message signalé
	"/api/v1/chat/messages/{message_id}/flagged-content": true,
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
	"/api/v1/auth/checkPhoneNumber": TierAuth,
	"/api/v1/auth/checkEmail":       TierAuth,

	// Création de compte — anti-spam
	"/api/v1/auth/createAccount": TierCreateAccount,

	// Opérations sensibles
	"/api/v1/auth/deleteAccount": TierSensitive,

	// chat-service — POST de messages = anti-spam léger via TierCreateAccount,
	// flag = sensible (modération support)
	"/api/v1/chat/threads/{thread_id}/messages": TierCreateAccount,
	"/api/v1/chat/messages/{message_id}/flag":   TierSensitive,
}
