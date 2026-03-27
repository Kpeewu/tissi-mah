package client

import "context"

// BookingClient est l'interface pour les appels vers booking-service.
type BookingClient interface {
	ConfirmPayment(ctx context.Context, bookingID string, transactionID string) error
	GetBookingDetails(ctx context.Context, bookingID string) (*BookingDetails, error)
}

// UserClient est l'interface pour les appels vers user-service.
type UserClient interface {
	GetUserByUserID(ctx context.Context, userID string) (*UserInfo, error)
}

// BookingDetails contient les détails d'une réservation nécessaires au payment-service.
type BookingDetails struct {
	BookingID   string
	TripID      string
	DriverID    string
	Status      string
	TotalAmount int
	ServiceFee  int
}

// UserInfo contient les informations utilisateur nécessaires au payout.
type UserInfo struct {
	UserID         string
	Name           string
	FirstName      string
	WithdrawNumber string
}
