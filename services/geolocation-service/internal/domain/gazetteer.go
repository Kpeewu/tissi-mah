// Package domain porte les règles métier du geolocation-service.
package domain

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// localities liste les localités importantes des pays desservis (Togo, Bénin, Ghana,
// Burkina Faso) ainsi que les quartiers de Lomé les plus utilisés comme points de
// rendez-vous. Elle ne sert qu'à CORRIGER une saisie : les coordonnées viennent
// toujours de Nominatim, interrogé de nouveau avec le nom corrigé. Une localité
// absente d'ici n'est donc jamais dégradée, elle ne bénéficie simplement pas de la
// correction.
var localities = []string{
	// Togo
	"Lomé", "Sokodé", "Kara", "Kpalimé", "Atakpamé", "Bassar", "Tsévié", "Aného",
	"Dapaong", "Mango", "Tchamba", "Niamtougou", "Badou", "Notsé", "Vogan", "Tabligbo",
	"Kandé", "Sotouboua", "Blitta", "Amlamé", "Bafilo", "Anié", "Adéta", "Afagnan",
	"Kévé", "Tohoun", "Agou", "Pagouda", "Danyi",
	// Quartiers et communes de l'agglomération de Lomé
	"Adidogomé", "Agoè", "Baguida", "Bè", "Tokoin", "Hédzranawoé", "Kodjoviakopé",
	"Nyékonakpoè", "Akodésséwa", "Avépozo",
	// Bénin
	"Cotonou", "Porto-Novo", "Parakou", "Djougou", "Bohicon", "Abomey", "Natitingou",
	"Lokossa", "Ouidah", "Kandi", "Malanville", "Savalou", "Comè", "Allada", "Aplahoué",
	"Nikki", "Banikoara", "Tanguiéta", "Dassa-Zoumé", "Kétou", "Pobè", "Sakété",
	"Grand-Popo", "Bembèrèkè", "Savè", "Tchaourou", "Covè", "Dogbo",
	// Ghana
	"Accra", "Kumasi", "Tamale", "Takoradi", "Sekondi", "Cape Coast", "Sunyani", "Ho",
	"Koforidua", "Techiman", "Tema", "Obuasi", "Wa", "Bolgatanga", "Bawku", "Nkawkaw",
	"Hohoe", "Aflao", "Winneba", "Kasoa", "Madina", "Ashaiman", "Berekum", "Dunkwa",
	"Axim", "Elmina", "Keta", "Kintampo", "Yendi", "Savelugu", "Salaga", "Damongo",
	"Tarkwa", "Prestea", "Konongo", "Ejura", "Mampong", "Nsawam", "Suhum", "Aburi",
	// Burkina Faso
	"Ouagadougou", "Bobo-Dioulasso", "Koudougou", "Banfora", "Ouahigouya", "Kaya",
	"Tenkodogo", "Fada N'Gourma", "Dédougou", "Dori", "Gaoua", "Ziniaré", "Réo", "Léo",
	"Yako", "Nouna", "Orodara", "Manga", "Pô", "Boulsa", "Koupéla", "Garango", "Houndé",
	"Diapaga", "Djibo", "Kongoussi", "Solenzo", "Tougan", "Bogandé", "Sindou",
	"Diébougou", "Gorom-Gorom", "Titao", "Toma", "Zorgho", "Boromo", "Pama",
}

// SuggestionThreshold est la similarité minimale pour proposer une correction.
// Mesurée sur des fautes réelles : « Skode » → « Sokodé » vaut 0,67, « Kpalme » →
// « Kpalimé » 0,73, alors que deux villes sans rapport tombent à 0,00.
const SuggestionThreshold = 0.6

// minSuggestionLength évite de corriger une saisie trop courte, où le hasard des
// bigrammes produit des rapprochements absurdes.
const minSuggestionLength = 3

// SuggestLocality retourne la localité connue la plus proche de la saisie et sa
// similarité, ou ("", 0) si aucune n'atteint le seuil.
func SuggestLocality(query string) (string, float64) {
	normalized := normalizeForMatch(query)
	if len([]rune(normalized)) < minSuggestionLength {
		return "", 0
	}

	best, bestScore := "", 0.0
	for _, locality := range localities {
		score := similarity(normalized, normalizeForMatch(locality))
		if score > bestScore {
			best, bestScore = locality, score
		}
	}
	if bestScore < SuggestionThreshold {
		return "", 0
	}
	return best, bestScore
}

// normalizeForMatch met en minuscules, retire les accents et la ponctuation :
// « Skodè » et « skode » doivent se comparer à l'identique.
func normalizeForMatch(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	cleaned, _, err := transform.String(t, strings.ToLower(strings.TrimSpace(s)))
	if err != nil {
		cleaned = strings.ToLower(strings.TrimSpace(s))
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			return r
		}
		return -1
	}, cleaned)
}

// similarity est le coefficient de Dice sur les bigrammes de caractères : robuste aux
// lettres manquantes, doublées ou interverties, et peu coûteux.
func similarity(a, b string) float64 {
	aBigrams, bBigrams := bigrams(a), bigrams(b)
	if len(aBigrams) == 0 || len(bBigrams) == 0 {
		return 0
	}

	counts := make(map[string]int, len(aBigrams))
	for _, bg := range aBigrams {
		counts[bg]++
	}
	common := 0
	for _, bg := range bBigrams {
		if counts[bg] > 0 {
			counts[bg]--
			common++
		}
	}
	return 2 * float64(common) / float64(len(aBigrams)+len(bBigrams))
}

func bigrams(s string) []string {
	r := []rune(s)
	if len(r) < 2 {
		if len(r) == 0 {
			return nil
		}
		return []string{string(r)}
	}
	out := make([]string, 0, len(r)-1)
	for i := 0; i < len(r)-1; i++ {
		out = append(out, string(r[i:i+2]))
	}
	return out
}
