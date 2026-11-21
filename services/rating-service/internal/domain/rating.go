package domain

import (
	"time"
)

// Nombre d'étoiles minimum et maximum autorisé
const (
	MinStars = 1
	MaxStars = 5
)

type Rating struct {
	RatingID      string
	RaterID       string
	UserRatedID   string
	NumberOfStars int16
	Comment       *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// Enrichissement inter-service (non persisté en base)
	RaterFirstName       string
	RaterLastName        string
	RaterProfileImageURL string
}

// Retourne le commentaire ou une chaîne vide si nil
func (r *Rating) GetComment() string {
	if r.Comment != nil {
		return *r.Comment
	}
	return ""
}

// Vérifie si la note a un commentaire
func (r *Rating) HasComment() bool {
	return r.Comment != nil && *r.Comment != ""
}

// Vérifie si le nombre d'étoiles est valide (entre 1 et 5)
func (r *Rating) IsValidStars() bool {
	return r.NumberOfStars >= MinStars && r.NumberOfStars <= MaxStars
}

// Vérifie qu'un utilisateur ne se note pas lui-même
func (r *Rating) IsSelfRating() bool {
	return r.RaterID == r.UserRatedID
}
