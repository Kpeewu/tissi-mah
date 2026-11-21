package otp_test

import (
	"regexp"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/otp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	re := regexp.MustCompile(`^[0-9]{6}$`)

	t.Run("format : 6 chiffres exactement", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			code, err := otp.Generate()
			require.NoError(t, err)
			assert.Len(t, code, 6, "itération %d : code %q", i, code)
			assert.Regexp(t, re, code, "itération %d : format invalide %q", i, code)
		}
	})

	t.Run("zéros de gauche préservés", func(t *testing.T) {
		// On ne peut pas forcer le zéro leading, mais sur 1000 tirages ça arrive statistiquement.
		// On vérifie juste que le format est toujours 6 chars (pas tronqué).
		for i := 0; i < 1000; i++ {
			code, err := otp.Generate()
			require.NoError(t, err)
			require.Len(t, code, 6)
		}
	})

	t.Run("entropie raisonnable sur 200 tirages", func(t *testing.T) {
		seen := make(map[string]int, 200)
		for i := 0; i < 200; i++ {
			code, err := otp.Generate()
			require.NoError(t, err)
			seen[code]++
		}
		// Avec un espace de 10^6 et 200 tirages, les doublons sont extrêmement improbables.
		// On tolère 1 doublon max pour absorber un hasard malheureux.
		duplicates := 0
		for _, count := range seen {
			if count > 1 {
				duplicates++
			}
		}
		assert.LessOrEqual(t, duplicates, 1, "trop de doublons (%d) pour 200 tirages sur 10^6", duplicates)
	})
}
