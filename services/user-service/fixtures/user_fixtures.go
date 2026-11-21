package fixtures

import (
	"time"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	"github.com/google/uuid"
)

type UserOption func(*domain.User)

func WithUserID(id string) UserOption {
	return func(u *domain.User) {
		u.UserID = id
	}
}

func WithAuthID(id string) UserOption {
	return func(u *domain.User) {
		u.AuthID = id
	}
}

func WithFirebaseID(id string) UserOption {
	return func(u *domain.User) {
		u.FirebaseID = id
	}
}

func WithName(name string) UserOption {
	return func(u *domain.User) {
		u.Name = name
	}
}

func WithFirstName(firstName string) UserOption {
	return func(u *domain.User) {
		u.FirstName = firstName
	}
}

func WithGender(gender string) UserOption {
	return func(u *domain.User) {
		u.Gender = gender
	}
}

func WithDateOfBirth(dob string) UserOption {
	return func(u *domain.User) {
		u.DateOfBirth = dob
	}
}

func WithBio(bio string) UserOption {
	return func(u *domain.User) {
		u.Bio = bio
	}
}

func WithProfileImage(url string) UserOption {
	return func(u *domain.User) {
		u.ProfileImageURL = url
		u.HasProfileImage = true
	}
}

func WithNoProfileImage() UserOption {
	return func(u *domain.User) {
		u.ProfileImageURL = ""
		u.HasProfileImage = false
	}
}

func WithDriver() UserOption {
	return func(u *domain.User) {
		u.IsDriver = true
	}
}

func WithPassenger() UserOption {
	return func(u *domain.User) {
		u.IsPassenger = true
	}
}

func WithTripPreferences(prefs []domain.TripPreference) UserOption {
	return func(u *domain.User) {
		u.TripPreferences = prefs
	}
}

func WithDeleted() UserOption {
	return func(u *domain.User) {
		now := time.Now().UTC()
		u.DeletedAt = &now
	}
}

func NewTestUser(opts ...UserOption) *domain.User {
	now := time.Now().UTC()
	id := uuid.New().String()

	user := &domain.User{
		UserID:      "user_" + id,
		AuthID:      "auth_" + id,
		FirebaseID:  "firebase_" + id,
		Name:        "Doe",
		FirstName:   "John",
		IsPassenger: true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	for _, opt := range opts {
		opt(user)
	}

	return user
}
