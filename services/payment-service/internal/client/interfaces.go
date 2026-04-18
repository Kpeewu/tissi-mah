package client

import "context"

// BookingClient est l'interface pour les appels vers booking-service.
type BookingClient interface {
	ConfirmPayment(ctx context.Context, bookingID string, transactionID string) error
	FailPayment(ctx context.Context, bookingID string, reason string) error
	GetBookingDetails(ctx context.Context, bookingID string) (*BookingDetails, error)
}

// UserClient est l'interface pour les appels vers user-service.
type UserClient interface {
	GetUserByUserID(ctx context.Context, userID string) (*UserInfo, error)
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
	TotalAmount      int
	ServiceFee       int
}

// UserInfo contient les informations utilisateur nécessaires au payout.
type UserInfo struct {
	UserID         string
	Name           string
	FirstName      string
	WithdrawNumber string
}
