package password

import "errors"

// ErrWeakPassword est retourné si le nouveau mot de passe ne respecte pas la politique.
var ErrWeakPassword = errors.New("weak password")

const minPasswordLength = 12

// ValidateStrength vérifie longueur >= 12 et présence des 4 classes de caractères.
func ValidateStrength(p string) error {
	if len(p) < minPasswordLength {
		return ErrWeakPassword
	}
	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasLower || !hasUpper || !hasDigit || !hasSpecial {
		return ErrWeakPassword
	}
	return nil
}
