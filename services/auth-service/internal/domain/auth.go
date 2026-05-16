package domain

import (
	"fmt"
	"time"
)

type Auth struct {
	AuthID            string
	FirebaseID        string
	Email             *string
	PhoneNumber       *string
	IsActive          bool
	IsSuspended       bool
	IsBanned          bool
	SuspensionEndDate *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

// Return user email account
func (a *Auth) GetEmail() string {
	if a.Email != nil {
		return *a.Email
	}
	return ""
}

// Return user phone number account
func (a *Auth) GetPhoneNumber() string {
	if a.PhoneNumber != nil {
		return *a.PhoneNumber
	}
	return ""
}

// Verify if the user have an email
func (a *Auth) HasEmail() bool {
	return a.Email != nil
}

// Verify if the user have a phone number
func (a *Auth) HasPhoneNumber() bool {
	return a.PhoneNumber != nil
}

// Verify if the user account has been deleted
func (a *Auth) IsDeleted() bool {
	return a.DeletedAt != nil
}

// Verify if the user account is actually suspended or banned
func (a *Auth) IsSuspendedNow() bool {
	if a.IsBanned {
		return true
	}

	if !a.IsSuspended {
		return false
	}

	if a.SuspensionEndDate == nil {
		return true
	}

	return time.Now().UTC().Before(*a.SuspensionEndDate)
}

// Ban permanently bans the account
func (a *Auth) Ban() {
	a.IsBanned = true
	a.IsSuspended = true
	a.SuspensionEndDate = nil
	a.UpdatedAt = time.Now().UTC()
}

// Verify if the user can login
func (a *Auth) CanLogin() bool {

	// Can't login if account deleted
	if a.IsDeleted() {
		return false
	}

	// Can't login if account not active
	if !a.IsActive {
		return false
	}

	// Can't login if account suspended
	if a.IsSuspendedNow() {
		return false
	}

	return true
}

// Suspend user account until a certain date
func (a *Auth) Suspend(endDate time.Time) {
	a.IsSuspended = true
	a.SuspensionEndDate = &endDate

	a.UpdatedAt = time.Now().UTC()
}

// Unlock user account
func (a *Auth) UnSuspend() {
	a.IsSuspended = false
	a.SuspensionEndDate = nil

	a.UpdatedAt = time.Now().UTC()
}

// Active user account
func (a *Auth) Activate() {
	a.IsActive = true
	a.UpdatedAt = time.Now().UTC()
}

// Deactivate user account
func (a *Auth) Deactivate() {
	a.IsActive = false
	a.UpdatedAt = time.Now().UTC()
}

// Delete user account
func (a *Auth) AnonymizeAndDelete() {

	a.FirebaseID = fmt.Sprintf("deleted_%s", a.AuthID)

	email := fmt.Sprintf("deleted_user_%s@anonymized.local", a.AuthID)
	a.Email = &email
	a.PhoneNumber = nil

	a.IsActive = false

	now := time.Now().UTC()
	a.DeletedAt = &now
	a.UpdatedAt = now
}
