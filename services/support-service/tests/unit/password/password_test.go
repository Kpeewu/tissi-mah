package password_test

import (
	"strings"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/password"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Hash / Verify
// =============================================================================

func TestHashAndVerify(t *testing.T) {
	t.Run("hash puis verify retourne true pour le bon mot de passe", func(t *testing.T) {
		h, err := password.Hash("MySecure123!")
		require.NoError(t, err)
		require.NotEmpty(t, h)

		ok, err := password.Verify(h, "MySecure123!")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("verify retourne false pour un mauvais mot de passe", func(t *testing.T) {
		h, err := password.Hash("MySecure123!")
		require.NoError(t, err)

		ok, err := password.Verify(h, "WrongPassword1!")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("hash deux fois le même mot de passe produit des hashs différents", func(t *testing.T) {
		h1, err := password.Hash("SamePassword1!")
		require.NoError(t, err)
		h2, err := password.Hash("SamePassword1!")
		require.NoError(t, err)
		assert.NotEqual(t, h1, h2, "le sel aléatoire doit rendre les hashs différents")
	})

	t.Run("verify retourne ErrInvalidHash pour un hash mal formé", func(t *testing.T) {
		_, err := password.Verify("not-a-valid-hash", "whatever")
		assert.ErrorIs(t, err, password.ErrInvalidHash)
	})

	t.Run("verify retourne ErrInvalidHash pour un hash tronqué", func(t *testing.T) {
		_, err := password.Verify("$argon2id$v=19$m=65536,t=1,p=4", "whatever")
		assert.ErrorIs(t, err, password.ErrInvalidHash)
	})

	t.Run("verify retourne ErrInvalidHash pour un sel base64 invalide", func(t *testing.T) {
		_, err := password.Verify("$argon2id$v=19$m=65536,t=1,p=4$!!!$abc", "whatever")
		assert.ErrorIs(t, err, password.ErrInvalidHash)
	})

	t.Run("hash d'une chaîne vide réussit et verify différencie", func(t *testing.T) {
		h, err := password.Hash("")
		require.NoError(t, err)
		ok, err := password.Verify(h, "")
		require.NoError(t, err)
		assert.True(t, ok)

		ok, err = password.Verify(h, "anything")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("hash supporte les caractères Unicode", func(t *testing.T) {
		h, err := password.Hash("Pâssw0rd!éöü")
		require.NoError(t, err)
		ok, err := password.Verify(h, "Pâssw0rd!éöü")
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

// =============================================================================
// ValidateStrength
// =============================================================================

func TestValidateStrength(t *testing.T) {
	valid := "Passw0rd!Test"
	require.NoError(t, password.ValidateStrength(valid))

	cases := []struct {
		name    string
		pwd     string
		wantErr bool
	}{
		{"valide (12 chars + 4 classes)", "Passw0rd!Test", false},
		{"valide (20 chars)", "Aa1!Aa1!Aa1!Aa1!Aa1!", false},
		{"trop court (11 chars)", "Passw0rd!Te", true},
		{"vide", "", true},
		{"manque majuscule", "passw0rd!test", true},
		{"manque minuscule", "PASSW0RD!TEST", true},
		{"manque chiffre", "Password!Test", true},
		{"manque caractère spécial", "Passw0rdTest", true},
		{"tout minuscule", "abcdefghijkl", true},
		{"tout chiffre", "123456789012", true},
		{"exactement 12 chars et conforme", "Abc123!xyzOP", false},
		{"unicode compte comme spécial", "Passw0rdéTest", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := password.ValidateStrength(tc.pwd)
			if tc.wantErr {
				assert.ErrorIs(t, err, password.ErrWeakPassword)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// GenerateTemporary
// =============================================================================

func TestGenerateTemporary(t *testing.T) {
	t.Run("longueur 16", func(t *testing.T) {
		p, err := password.GenerateTemporary()
		require.NoError(t, err)
		assert.Len(t, p, 16)
	})

	t.Run("satisfait toujours la policy", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			p, err := password.GenerateTemporary()
			require.NoError(t, err)
			assert.NoError(t, password.ValidateStrength(p),
				"password %q (itération %d) doit satisfaire la policy", p, i)
		}
	})

	t.Run("entropie : 100 tirages uniques", func(t *testing.T) {
		seen := make(map[string]struct{}, 100)
		for i := 0; i < 100; i++ {
			p, err := password.GenerateTemporary()
			require.NoError(t, err)
			_, duplicate := seen[p]
			assert.False(t, duplicate, "password %q répété (itération %d)", p, i)
			seen[p] = struct{}{}
		}
	})

	t.Run("contient bien les 4 classes", func(t *testing.T) {
		p, err := password.GenerateTemporary()
		require.NoError(t, err)

		hasLower := strings.ContainsAny(p, "abcdefghijklmnopqrstuvwxyz")
		hasUpper := strings.ContainsAny(p, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
		hasDigit := strings.ContainsAny(p, "0123456789")
		hasSpecial := strings.ContainsAny(p, "!@#$%^&*()-_=+[]{}")

		assert.True(t, hasLower, "doit contenir une minuscule")
		assert.True(t, hasUpper, "doit contenir une majuscule")
		assert.True(t, hasDigit, "doit contenir un chiffre")
		assert.True(t, hasSpecial, "doit contenir un caractère spécial")
	})
}
