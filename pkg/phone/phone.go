package phone

import (
	"strings"
	"unicode"
)

// Result contient le numéro normalisé et les métadonnées FedaPay associées.
type Result struct {
	E164        string // ex. "+22890123456" — format stocké en DB
	LocalNumber string // ex. "90123456"     — envoyé à FedaPay (champ phone_number.number)
	CountryCode string // ex. "tg", "bj"     — envoyé à FedaPay (champ phone_number.country)
}

// validOperators mappe le code pays ISO → liste des modes FedaPay valides.
var validOperators = map[string][]string{
	"tg": {"togocel"},
	"bj": {"mtn_open", "moov", "sbin"},
}

// countryPrefixes mappe le préfixe international (sans +) → code pays ISO.
// L'ordre d'évaluation n'a pas d'importance : les préfixes sont distincts.
var countryPrefixes = map[string]string{
	"228": "tg",
	"229": "bj",
}

// localLen est la longueur attendue du numéro local pour tous les pays UEMOA supportés.
const localLen = 8

// Normalize normalise un numéro Mobile Money en acceptant :
//   - E.164     : +228XXXXXXXX, +229XXXXXXXX
//   - Format 00 : 00228XXXXXXXX, 00229XXXXXXXX
//   - 8 chiffres nus (fallback Togo — rétrocompatibilité des enregistrements existants)
//
// Retourne (Result, false) si le format est invalide ou le pays non supporté.
func Normalize(raw string) (Result, bool) {
	digits := extractDigits(raw)
	if digits == "" {
		return Result{}, false
	}

	// Normalise le préfixe 00XX → XXXX pour un traitement uniforme
	if strings.HasPrefix(digits, "00") {
		digits = digits[2:]
	}

	// Détection par préfixe international
	for prefix, cc := range countryPrefixes {
		if strings.HasPrefix(digits, prefix) {
			local := digits[len(prefix):]
			if len(local) == localLen {
				return Result{
					E164:        "+" + prefix + local,
					LocalNumber: local,
					CountryCode: cc,
				}, true
			}
		}
	}

	// Fallback rétrocompat : 8 chiffres nus → Togo
	if len(digits) == localLen {
		return Result{
			E164:        "+228" + digits,
			LocalNumber: digits,
			CountryCode: "tg",
		}, true
	}

	return Result{}, false
}

// IsValidOperator retourne vrai si l'opérateur est accepté par FedaPay pour ce pays.
func IsValidOperator(countryCode, operator string) bool {
	for _, op := range validOperators[countryCode] {
		if op == operator {
			return true
		}
	}
	return false
}

// extractDigits extrait les chiffres d'une chaîne en tolérant +, espace, tiret et point.
// Retourne "" si un caractère invalide est rencontré.
func extractDigits(raw string) string {
	s := strings.TrimSpace(raw)
	var b strings.Builder
	for i, r := range s {
		switch {
		case unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '+' && i == 0:
			// + en tête ignoré — on travaille sur les chiffres bruts
		case r == ' ' || r == '-' || r == '.':
			// séparateurs tolérés
		default:
			return ""
		}
	}
	return b.String()
}
