package client

import (
	"context"
	"time"
)

// BookingClient est l'interface pour les appels vers booking-service.
type BookingClient interface {
	// ConfirmPayment rattache le paiement à la réservation et retourne son nouveau statut
	// (approved si auto-approbation, pendingApproval sinon). Si la réservation avait été libérée
	// (paymentFailed), booking-service tente de re-réserver les places : ErrorNoSeatsAvailable
	// si le trajet est complet. ErrorPaymentAlreadyProcessed si la réservation est déjà confirmée.
	ConfirmPayment(ctx context.Context, bookingID string, transactionID string) (string, error)
	FailPayment(ctx context.Context, bookingID string, reason string) error
	GetBookingDetails(ctx context.Context, bookingID string) (*BookingDetails, error)
}

// UserClient est l'interface pour les appels vers user-service.
type UserClient interface {
	GetUserByUserID(ctx context.Context, userID string) (*UserInfo, error)
	// GetUserIDByFirebaseID résout le UserID interne d'un utilisateur à partir de son Firebase UID.
	GetUserIDByFirebaseID(ctx context.Context, firebaseID string) (string, error)
}

// SupportClient est l'interface pour les appels vers support-service.
type SupportClient interface {
	GetSupportUserByID(ctx context.Context, userID string) (*SupportUserInfo, error)
}

// SupportUserInfo contient les informations d'un agent support nécessaires au payout manuel.
type SupportUserInfo struct {
	UserID    string
	FirstName string
	LastName  string
	Role      string // "admin" | "support"
	IsActive  bool
}

// BookingDetails contient les détails d'une réservation nécessaires au payment-service.
type BookingDetails struct {
	BookingID        string
	BookingReference string
	TripID           string
	PassengerID      string
	DriverID         string
	Status           string
	PaymentMethod    string
	Subtotal         int
	TotalAmount      int
	ServiceFee       int
	CreatedAt        time.Time
}

// UserInfo contient les informations utilisateur nécessaires au payout.
type UserInfo struct {
	UserID         string
	Name           string
	FirstName      string
	WithdrawNumber string
}
