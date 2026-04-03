package client

import (
	"context"
)

// UserInfo contient les données utilisateur nécessaires au notification-service.
type UserInfo struct {
	UserID       string
	Name         string
	FirstName    string
	Email        string
	PhoneNumber  string
	LanguageCode string
}

// UserClient récupère les informations utilisateur via gRPC.
type UserClient interface {
	GetUserByUserID(ctx context.Context, userID string) (*UserInfo, error)
	Close() error
}

// PushClient envoie des notifications push via gRPC.
type PushClient interface {
	SendPush(ctx context.Context, title, body, token string, data map[string]string) (bool, string, error)
	SendPushMulticast(ctx context.Context, title, body string, tokens []string, data map[string]string) (int32, int32, error)
	Close() error
}

// EmailClient envoie des emails via gRPC.
type EmailClient interface {
	SendEmail(ctx context.Context, to, subject, bodyText, bodyHTML string) (bool, string, error)
	Close() error
}
