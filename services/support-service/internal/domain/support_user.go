package domain

import "time"

const (
	RoleAdmin   = "admin"
	RoleSupport = "support"
)

// IsValidRole indique si le rôle fourni fait partie des rôles autorisés.
func IsValidRole(role string) bool {
	return role == RoleAdmin || role == RoleSupport
}

// SupportUser représente un agent du back-office (admin ou support).
type SupportUser struct {
	UserID             string
	Email              string
	PasswordHash       string
	FirstName          string
	LastName           string
	Role               string
	IsActive           bool
	MustChangePassword bool
	EmailChangedAt     *time.Time
	PasswordChangedAt  time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
}
