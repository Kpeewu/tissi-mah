package domain

import (
	"fmt"
	"time"
)

// TripPreference représente une préférence de trajet individuelle
type TripPreference struct {
	Preference string `bson:"preference"`
	IsAllowed  bool   `bson:"is_allowed"`
}

// User représente l'entité profil utilisateur stockée dans MongoDB.
type User struct {
	UserID                     string           `bson:"user_id"`
	AuthID                     string           `bson:"auth_id"`
	FirebaseID                 string           `bson:"firebase_id"`
	Name                       string           `bson:"name"`
	FirstName                  string           `bson:"first_name"`
	Gender                     string           `bson:"gender"`
	DateOfBirth                string           `bson:"date_of_birth"`
	Bio                        string           `bson:"bio"`
	HasProfileImage            bool             `bson:"has_profile_image"`
	ProfileImageURL            string           `bson:"profile_image_url"`
	IsDriver                   bool             `bson:"is_driver"`
	IsPassenger                bool             `bson:"is_passenger"`
	IsDriverProfileVerified    bool             `bson:"is_driver_profile_verified"`
	IsPassengerProfileVerified bool             `bson:"is_passenger_profile_verified"`
	WithdrawNumber             string           `bson:"withdraw_number,omitempty"`
	TripPreferences            []TripPreference `bson:"trip_preferences,omitempty"`
	CreatedAt                  time.Time        `bson:"created_at"`
	UpdatedAt                  time.Time        `bson:"updated_at"`
	DeletedAt                  *time.Time       `bson:"deleted_at,omitempty"`
}

// EnableDriverAccount active le statut conducteur
func (u *User) EnableDriverAccount() {
	u.IsDriver = true
	u.UpdatedAt = time.Now().UTC()
}

// DisableDriverAccount désactive le statut conducteur
func (u *User) DisableDriverAccount() {
	u.IsDriver = false
	u.UpdatedAt = time.Now().UTC()
}

// SetTripPreferences met à jour les préférences de trajet
func (u *User) SetTripPreferences(preferences []TripPreference) {
	u.TripPreferences = preferences
	u.UpdatedAt = time.Now().UTC()
}

// IsDeleted vérifie si le profil a été soft-deleted
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

// AnonymizeAndDelete anonymise les données personnelles et marque le profil comme supprimé
func (u *User) AnonymizeAndDelete() {
	u.FirebaseID = ""
	u.Name = fmt.Sprintf("deleted_%s", u.UserID[:8])
	u.FirstName = ""
	u.Gender = ""
	u.DateOfBirth = ""
	u.Bio = ""
	u.ProfileImageURL = ""
	u.HasProfileImage = false
	u.WithdrawNumber = ""
	u.TripPreferences = nil
	u.IsDriver = false
	u.IsPassenger = false
	u.IsDriverProfileVerified = false
	u.IsPassengerProfileVerified = false

	now := time.Now().UTC()
	u.DeletedAt = &now
	u.UpdatedAt = now
}
