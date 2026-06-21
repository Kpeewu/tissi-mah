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
	"/api/v1/file/uploadIdDocument":       true,
	"/api/v1/file/uploadVehicleDocuments": true,
	"/api/v1/file/changeDocument":         true,
	"/api/v1/file/getDocument":            true,
	"/api/v1/file/deleteFile":             true,

	// vehicle-service
	"/api/v1/vehicle/add":             true,
	"/api/v1/vehicle/update":          true,
	"/api/v1/vehicle/delete":          true,
	"/api/v1/vehicle/details":         true,
	"/api/v1/vehicle/getUserVehicles": true,

	// kyc-service (user-facing)
	"/api/v1/kyc/inquiries/add":        true,
	"/api/v1/kyc/inquiries/getInquiry": true,
	"/api/v1/kyc/me/getStatus":         true,
	"/api/v1/kyc/inquiries/resume":     true,
	// kyc-service — webhook et health sont publics (pas de JWT)
	// kyc-service admin → SupportProtectedRoutes

	// booking-service (internal/* et health sont publics)
	"/api/v1/booking/createBooking":              true,
	"/api/v1/booking/getBookingDetails":          true,
	"/api/v1/booking/getPassengerBookings":       true,
	"/api/v1/booking/getDriverTripBookings":      true,
	"/api/v1/booking/getDriverPendingBookings":   true,
	"/api/v1/booking/getActivePassengerSummaries": true,
	"/api/v1/booking/approveBooking":             true,
	"/api/v1/booking/rejectBooking":              true,
	"/api/v1/booking/cancelBooking":              true,
	"/api/v1/booking/reportNoShow":               true,
	"/api/v1/booking/confirmPayment":             true,

	// payment-service (protégé)
	"/api/v1/payment/createPayment":       true,
	"/api/v1/payment/getPaymentStatus":    true,
	"/api/v1/payment/getPaymentByBooking": true,
	"/api/v1/payment/getRefundStatus":     true,
	"/api/v1/payment/getPayoutStatus":     true,
	"/api/v1/payment/getDriverPayouts":    true,
	// payment-service — webhooks, internal et health sont publics

	// trips-service
	"/api/v1/trip/driver/createTrip":                true,
	"/api/v1/trip/driver/createRecurringTrip":       true,
	"/api/v1/trip/driver/getTripsPreviews":          true,
	"/api/v1/trip/driver/getCompletedTripsPreviews": true,
	"/api/v1/trip/driver/changeTripDateAndTime":     true,
	"/api/v1/trip/driver/changeTripVehicle":         true,
	"/api/v1/trip/driver/changeTripAllowances":      true,
	"/api/v1/trip/driver/activeAutoApprouve":        true,
	"/api/v1/trip/driver/startTrip":                 true,
	"/api/v1/trip/driver/endTrip":                   true,
	"/api/v1/trip/driver/confirmWaypointArrival":    true,
	"/api/v1/trip/driver/confirmWaypointDeparture":  true,
	"/api/v1/trip/driver/cancelWaypoint":            true,
	"/api/v1/trip/driver/cancelTrip":                true,
	"/api/v1/trip/driver/getTripDetails":            true,

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
	"/api/v1/support/admin/agents/activate":   true,
	"/api/v1/support/admin/agents/delete":     true,

	// payment-service — actions support
	"/api/v1/payment/support/triggerManualPayout": true,

	// chat-service — accès support au contenu déchiffré d'un message signalé
	"/api/v1/chat/messages/{message_id}/flagged-content": true,

	// kyc-service — validation et consultation des revues de documents (réservé support)
	"/api/v1/kyc/admin/reviews/getReviews":  true,
	"/api/v1/kyc/admin/reviews/getReview":   true,
	"/api/v1/kyc/admin/reviews/override":    true,
	"/api/v1/kyc/admin/validateDocument":    true,
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
