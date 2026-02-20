package domain

type UserPreview struct {
	AuthID          string
	UserID          string
	Name            string
	FirstName       string
	Email           *string
	PhoneNumber     *string
	ProfilePhotoURL *string
}
