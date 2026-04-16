package domain

import (
	"regexp"
	"strings"
)

// Regex de validation
var (
	// Email : format basique — local@domain.tld
	// Pas de RFC 5322 complète, mais couvre les cas réels
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	// Téléphone E.164 : + suivi de 7 à 15 chiffres
	phoneRegex = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)
)

const maxEmailLength = 254 // RFC 5321

// ValidateEmail vérifie le format d'une adresse email.
// Retourne une erreur descriptive si le format est invalide, nil sinon.
func ValidateEmail(email string) error {
	if len(email) > maxEmailLength {
		return ErrEmailTooLong
	}
	if !emailRegex.MatchString(email) {
		return ErrEmailInvalidFormat
	}
	return nil
}

// ValidatePhone vérifie le format d'un numéro de téléphone (E.164).
// Retourne une erreur descriptive si le format est invalide, nil sinon.
func ValidatePhone(phone string) error {
	if !phoneRegex.MatchString(phone) {
		return ErrPhoneInvalidFormat
	}
	return nil
}

// NormalizeEmail met l'email en minuscules et supprime les espaces.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizePhone supprime les espaces du numéro de téléphone.
func NormalizePhone(phone string) string {
	return strings.TrimSpace(phone)
}
