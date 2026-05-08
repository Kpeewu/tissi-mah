package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// StreamName est le nom du Redis Stream utilisé pour les événements de notification.
const StreamName = "notification-events"

// Event types
const (
	BookingConfirmed            = "BOOKING_CONFIRMED"
	BookingRejected             = "BOOKING_REJECTED"
	BookingCancelledByDriver    = "BOOKING_CANCELLED_BY_DRIVER"
	BookingCancelledByPassenger = "BOOKING_CANCELLED_BY_PASSENGER"
	TripCancelled               = "TRIP_CANCELLED"
	TripModified                = "TRIP_MODIFIED"
	TripDepartureReminder       = "TRIP_DEPARTURE_REMINDER"
	WaypointCanceled            = "WAYPOINT_CANCELED"
	NoShowAtDeparture           = "NO_SHOW_AT_DEPARTURE"
	RatingReceived              = "RATING_RECEIVED"
	DocumentValidated           = "DOCUMENT_VALIDATED"
	DocumentRejected            = "DOCUMENT_REJECTED"
	DocumentExpiringSoon        = "DOCUMENT_EXPIRING_SOON"
	SupportTicketReplied        = "SUPPORT_TICKET_REPLIED"
	NewMessage                  = "NEW_MESSAGE"
	PaymentCompleted            = "PAYMENT_COMPLETED"
	PaymentFailed               = "PAYMENT_FAILED"
	RefundProcessed             = "REFUND_PROCESSED"
	RefundCompleted             = "REFUND_COMPLETED"
	RefundFailed                = "REFUND_FAILED"
	DriverPaymentLaunched       = "DRIVER_PAYMENT_LAUNCHED"
	TripFull                    = "TRIP_FULL"
	TripStarted                 = "TRIP_STARTED"
	TripEnded                   = "TRIP_ENDED"
	Welcome                     = "WELCOME"
	AccountSuspended            = "ACCOUNT_SUSPENDED"
	NewBookingRequest           = "NEW_BOOKING_REQUEST"
	KycApproved                 = "KYC_APPROVED"
	KycRejected                 = "KYC_REJECTED"
	DriverProfileVerified       = "DRIVER_PROFILE_VERIFIED"
)

// Reference types
const (
	RefUser          = "user"
	RefBooking       = "booking"
	RefTrip          = "trip"
	RefDocument      = "document"
	RefPayment       = "payment"
	RefRefund        = "refund"
	RefRating        = "rating"
	RefSupportTicket = "support_ticket"
	RefChatMessage   = "chat_message"
)

// Event représente un événement de notification publié dans le Redis Stream.
type Event struct {
	EventID       string            `json:"event_id"`
	EventType     string            `json:"event_type"`
	UserID        string            `json:"user_id,omitempty"`
	UserIDs       []string          `json:"user_ids,omitempty"`
	ReferenceID   string            `json:"reference_id"`
	ReferenceType string            `json:"reference_type"`
	Payload       map[string]string `json:"payload"`
}

// Publish envoie un événement dans le Redis Stream notification-events.
// Si EventID est vide, un UUID est généré automatiquement.
func Publish(ctx context.Context, rdb *redis.Client, event Event) error {
	if event.EventID == "" {
		event.EventID = uuid.New().String()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("notification: marshal event: %w", err)
	}

	return rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamName,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Err()
}
