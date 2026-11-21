package domain_test

import (
	"testing"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

const sokode = "Sokodé"

func TestSuggestLocality_CorrigeLesFautesDeFrappe(t *testing.T) {
	cases := map[string]string{
		"Skode":      sokode,
		"Skodè":      sokode,
		"Sokod":      sokode,
		"Sokodee":    sokode,
		"Kpalme":     "Kpalimé",
		"Atakpam":    "Atakpamé",
		"Lomme":      "Lomé",
		"Karra":      "Kara",
		"Tsevie":     "Tsévié",
		"kumasy":     "Kumasi",
		"Ouagadougo": "Ouagadougou",
	}
	for input, want := range cases {
		got, score := domain.SuggestLocality(input)
		assert.Equal(t, want, got, "saisie %q", input)
		assert.GreaterOrEqual(t, score, domain.SuggestionThreshold, "saisie %q", input)
	}
}

func TestSuggestLocality_NeCorrigePasCeQuiEstDejaBon(t *testing.T) {
	// Une saisie exacte reste inchangée : la correction ne se déclenche que si
	// Nominatim n'a rien trouvé, mais elle doit rester cohérente.
	for _, exact := range []string{"Lomé", sokode, "Accra", "Cotonou"} {
		got, _ := domain.SuggestLocality(exact)
		assert.Equal(t, exact, got)
	}
}

func TestSuggestLocality_NInventeRien(t *testing.T) {
	for _, input := range []string{"xyzzy", "boulangerie", "azertyuiop", "12345"} {
		got, score := domain.SuggestLocality(input)
		assert.Empty(t, got, "saisie %q ne doit rien suggérer (score %.2f)", input, score)
	}
}

func TestSuggestLocality_IgnoreLesSaisiesTropCourtes(t *testing.T) {
	for _, input := range []string{"", "L", "Lo"} {
		got, _ := domain.SuggestLocality(input)
		assert.Empty(t, got, "saisie %q", input)
	}
}
