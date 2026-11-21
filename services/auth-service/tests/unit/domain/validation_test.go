package domain_test

import (
	"strings"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ValidateEmail
// =============================================================================

func TestValidateEmail(t *testing.T) {
	t.Run("valide - formats classiques", func(t *testing.T) {
		valids := []string{
			"user@example.com",
			"user.name@example.com",
			"user+tag@example.com",
			"user@sub.domain.com",
			"u@example.co",
			"first.last@company.tg",
		}
		for _, email := range valids {
			assert.NoError(t, domain.ValidateEmail(email), "devrait accepter: %s", email)
		}
	})

	t.Run("invalide - format incorrect", func(t *testing.T) {
		invalids := []string{
			"",
			"noatsign",
			"@nodomain.com",
			"user@",
			"user@.com",
			"user@domain",
			"user @example.com",
			"us er@example.com",
			"user@@example.com",
		}
		for _, email := range invalids {
			err := domain.ValidateEmail(email)
			assert.ErrorIs(t, err, domain.ErrEmailInvalidFormat, "devrait rejeter: %q", email)
		}
	})

	t.Run("invalide - dépasse 254 caractères", func(t *testing.T) {
		longLocal := strings.Repeat("a", 243)
		longEmail := longLocal + "@example.com" // 243 + 12 = 255
		err := domain.ValidateEmail(longEmail)
		assert.ErrorIs(t, err, domain.ErrEmailTooLong)
	})

	t.Run("valide - exactement 254 caractères", func(t *testing.T) {
		longLocal := strings.Repeat("a", 242)
		email := longLocal + "@example.com" // 242 + 12 = 254
		require.Len(t, email, 254)
		assert.NoError(t, domain.ValidateEmail(email))
	})
}

// =============================================================================
// ValidatePhone
// =============================================================================

func TestValidatePhone(t *testing.T) {
	t.Run("valide - numéros internationaux E.164", func(t *testing.T) {
		valids := []string{
			"+22890123456",   // Togo (8 chiffres)
			"+221781234567",  // Sénégal (9 chiffres)
			"+2250712345678", // Côte d'Ivoire (10 chiffres)
			"+33612345678",   // France (9 chiffres)
			"+14155551234",   // USA (10 chiffres)
			"+8613812345678", // Chine (11 chiffres)
			"+1234567",       // Minimum 7 chiffres
		}
		for _, phone := range valids {
			assert.NoError(t, domain.ValidatePhone(phone), "devrait accepter: %s", phone)
		}
	})

	t.Run("invalide - format incorrect", func(t *testing.T) {
		invalids := []string{
			"",
			"22890123456",        // Manque le +
			"+0123456789",        // Commence par 0 après le +
			"+123",               // Trop court (< 7 chiffres)
			"+1234567890123456",  // Trop long (> 15 chiffres)
			"+228 90 12 34 56",   // Espaces (après trim, testé via NormalizePhone)
			"+228-90-123-456",    // Tirets
			"+abcdefghij",        // Lettres
			"phone",              // Texte
		}
		for _, phone := range invalids {
			err := domain.ValidatePhone(phone)
			assert.ErrorIs(t, err, domain.ErrPhoneInvalidFormat, "devrait rejeter: %q", phone)
		}
	})
}

// =============================================================================
// NormalizeEmail
// =============================================================================

func TestNormalizeEmail(t *testing.T) {
	t.Run("met en minuscules et supprime les espaces", func(t *testing.T) {
		assert.Equal(t, "user@example.com", domain.NormalizeEmail("  User@Example.COM  "))
	})

	t.Run("chaîne vide reste vide", func(t *testing.T) {
		assert.Equal(t, "", domain.NormalizeEmail(""))
	})

	t.Run("espaces uniquement donne une chaîne vide", func(t *testing.T) {
		assert.Equal(t, "", domain.NormalizeEmail("   "))
	})
}

// =============================================================================
// NormalizePhone
// =============================================================================

func TestNormalizePhone(t *testing.T) {
	t.Run("supprime les espaces en début et fin", func(t *testing.T) {
		assert.Equal(t, "+22890123456", domain.NormalizePhone("  +22890123456  "))
	})

	t.Run("chaîne vide reste vide", func(t *testing.T) {
		assert.Equal(t, "", domain.NormalizePhone(""))
	})
}
