package filter

import (
	"regexp"
	"strings"
)

const redactToken = "★★★"

// PIIFilter détecte et masque les données personnelles dans un message texte :
// numéros de téléphone (Togo/Ghana/Bénin/Burkina Faso), adresses email,
// URLs (sauf Google Maps), pseudos réseaux sociaux (@user, t.me/, wa.me/).
type PIIFilter struct {
	patterns []*regexp.Regexp
	googleMapsRe *regexp.Regexp
}

// NewPIIFilter construit le filtre avec tous les patterns activés.
func NewPIIFilter() *PIIFilter {
	return &PIIFilter{
		patterns: []*regexp.Regexp{
			// ---- Numéros de téléphone ----
			// Togo (+228) : 8 chiffres, prefixes 70-79, 90-99, 22-29
			rePhone(`(?:\+?228[\s\-.]?)?(?:[79][0-9]|2[2-9])[\s\-.]?[0-9]{3}[\s\-.]?[0-9]{3}`),
			// Ghana (+233) : 9 chiffres, prefixes 02x/03x/05x
			rePhone(`(?:\+?233[\s\-.]?)?[023][0-9][\s\-.]?[0-9]{3}[\s\-.]?[0-9]{4}`),
			// Bénin (+229) : 8 chiffres, prefixes 61-69, 91-99, 51-59, 01, 21
			rePhone(`(?:\+?229[\s\-.]?)?(?:[0-9]{2})[\s\-.]?[0-9]{2}[\s\-.]?[0-9]{2}[\s\-.]?[0-9]{2}`),
			// Burkina Faso (+226) : 8 chiffres, prefixes 50, 60-79
			rePhone(`(?:\+?226[\s\-.]?)?(?:5[0]|[67][0-9])[\s\-.]?[0-9]{2}[\s\-.]?[0-9]{2}[\s\-.]?[0-9]{2}`),
			// Format international générique +XXX...
			rePhone(`\+(?:[0-9][\s\-.]?){8,14}[0-9]`),

			// ---- Email ----
			regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`),

			// ---- URLs avec scheme (http/https/ftp/ftps/sftp/ws/wss + tout
			// scheme custom type whatsapp://, tg://, viber://, fb-messenger://) ----
			regexp.MustCompile(`(?i)(?:https?|ftps?|sftp|wss?)://[^\s<>"']+`),
			regexp.MustCompile(`(?i)\b[a-z][a-z0-9+\-]{1,15}://[^\s<>"']+`),

			// ---- Schemes mobile (tel:, sms:, mailto:, callto:) ----
			regexp.MustCompile(`(?i)\b(?:tel|sms|mailto|callto):[^\s<>"']+`),

			// ---- URLs sans scheme : www.example.com, raccourcisseurs ----
			regexp.MustCompile(`(?i)\bwww\.[a-z0-9\-]+\.[a-z]{2,}(?:[/?#][^\s]*)?`),
			// Raccourcisseurs courants (forme bare-domain)
			regexp.MustCompile(`(?i)\b(?:bit\.ly|tinyurl\.com|t\.co|ow\.ly|is\.gd|buff\.ly|goo\.gl|cutt\.ly|rebrand\.ly|short\.io|rb\.gy|s\.id)/[^\s]+`),
			// Domaines bruts type "exemple.com/path", "site.tg", "page.gh" —
			// limite aux TLD les plus courants pour minimiser les faux positifs.
			regexp.MustCompile(`(?i)\b[a-z0-9](?:[a-z0-9\-]{0,61}[a-z0-9])?\.(?:com|net|org|io|app|me|info|biz|tg|gh|bj|bf|fr|ci|sn|ml|ne)(?:[/?#][^\s]*)?\b`),

			// ---- Réseaux sociaux (formes compactes) ----
			regexp.MustCompile(`(?i)\bwa\.me/[0-9+]+`),
			regexp.MustCompile(`(?i)\bt\.me/[a-z0-9_]+`),
			regexp.MustCompile(`(?i)\b(?:fb|m|messenger)\.com/[a-z0-9_.\-]+`),
			regexp.MustCompile(`(?i)\b(?:instagram|tiktok|snapchat|twitter|x|threads)\.com/[a-z0-9_.\-]+`),
			// Handle @username (au moins 3 chars, pas de faux positifs sur @5h)
			regexp.MustCompile(`@[a-z0-9_]{3,30}\b`),
		},
		// Whitelist Google Maps (URL complète, avec ou sans scheme) :
		// ne pas masquer les liens partagés pour indiquer un point de
		// rendez-vous. Couvre: https://maps.google.com/..., maps.app.goo.gl/...,
		// google.com/maps/..., goo.gl/maps/..., maps.app.goo.gl/...
		googleMapsRe: regexp.MustCompile(
			`(?i)(?:https?://)?` +
				`(?:maps\.google\.[a-z]{2,3}` +
				`|(?:www\.)?google\.[a-z]{2,3}/maps` +
				`|maps\.app\.goo\.gl` +
				`|goo\.gl/maps)` +
				`[^\s]*`),
	}
}

// Filter analyse content, masque les PII avec ★★★ et retourne le texte
// filtré ainsi qu'un booléen indiquant si des redactions ont eu lieu.
func (f *PIIFilter) Filter(content string) (filtered string, hasRedaction bool) {
	result := content

	// Extraire et protéger d'abord les URLs Google Maps pour qu'elles ne
	// soient pas masquées par les patterns URL génériques.
	mapsMatches := f.googleMapsRe.FindAllString(result, -1)
	placeholders := make(map[string]string, len(mapsMatches))
	for i, m := range mapsMatches {
		key := "\x00MAPS" + strings.Repeat(string(rune('A'+i)), len(m)) + "\x00"
		placeholders[key] = m
		result = strings.Replace(result, m, key, 1)
	}

	// Appliquer chaque pattern de masquage
	for _, re := range f.patterns {
		newResult := re.ReplaceAllString(result, redactToken)
		if newResult != result {
			hasRedaction = true
			result = newResult
		}
	}

	// Restaurer les URLs Google Maps whitelistées
	for key, original := range placeholders {
		result = strings.Replace(result, key, original, 1)
	}

	return result, hasRedaction
}

func rePhone(pattern string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|[\s,;:(])` + pattern + `(?:$|[\s,;:)])`)
}
