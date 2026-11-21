package filter

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// TextFilterResult est le résultat du filtre local sur un texte.
type TextFilterResult struct {
	Score   float32
	Matched string
}

// TextFilterImpl applique un filtre local multilingue (FR/EN/DE/IT/ES) avec
// détection des tentatives de contournement (leet-speak, séparateurs, substitutions).
type TextFilterImpl struct {
	// wordPatterns est appliqué sur le texte entièrement normalisé (leet + séparateurs supprimés).
	wordPatterns []*regexp.Regexp
	// rawPatterns est appliqué sur le texte brut (juste leet + minuscules)
	// pour détecter des motifs comme n*gre, p.rn, f*ck où un caractère remplace une lettre.
	rawPatterns []*regexp.Regexp
}

// NewTextFilter construit un filtre prêt à l'emploi.
func NewTextFilter() *TextFilterImpl {
	words := buildWordList()
	wordPatterns := make([]*regexp.Regexp, 0, len(words))
	for _, w := range words {
		wordPatterns = append(wordPatterns, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(w)+`\b`))
	}

	rawPatterns := buildRawPatterns()

	return &TextFilterImpl{
		wordPatterns: wordPatterns,
		rawPatterns:  rawPatterns,
	}
}

// Analyze retourne un score 0.0-1.0 et le premier terme correspondant.
//
// Pipeline :
//  1. Normalisation complète (leet + séparateurs) → wordPatterns
//  2. Normalisation légère (leet uniquement)       → rawPatterns (remplacements de lettre)
func (f *TextFilterImpl) Analyze(text string) TextFilterResult {
	// Passe 1 : texte entièrement normalisé
	norm := fullNormalize(text)
	for _, p := range f.wordPatterns {
		if m := p.FindString(norm); m != "" {
			return TextFilterResult{Score: 0.95, Matched: m}
		}
	}

	// Passe 2 : leet uniquement (ex: n*gre, p.rn, sh!t)
	leet := leetNormalize(text)
	for _, p := range f.rawPatterns {
		if m := p.FindString(leet); m != "" {
			return TextFilterResult{Score: 0.95, Matched: m}
		}
	}

	return TextFilterResult{Score: 0.0}
}

// ─── Normalisation ──────────────────────────────────────────────────────────

// leetTable remplace les substitutions leet-speak courantes.
var leetReplacer = strings.NewReplacer(
	"0", "o",
	"1", "i",
	"3", "e",
	"4", "a",
	"5", "s",
	"6", "g",
	"7", "t",
	"8", "b",
	"9", "q",
	"@", "a",
	"$", "s",
	"!", "i",
	"+", "t",
	"|", "i",
	"€", "e",
	"(", "c",
)

// separatorBetweenLetters supprime les caractères non-alphanumériques uniques
// intercalés entre deux lettres (f.u.c.k → fuck, n-e-g-r-e → negre).
// Appliqué plusieurs fois pour gérer les chaînes longues.
var separatorBetween = regexp.MustCompile(`([a-z])[^a-z0-9]{1,2}([a-z])`)

func fullNormalize(s string) string {
	s = removeDiacritics(s)
	s = strings.ToLower(s)
	s = leetReplacer.Replace(s)
	// Appliquer la suppression des séparateurs plusieurs fois
	for range 5 {
		prev := s
		s = separatorBetween.ReplaceAllString(s, "$1$2")
		if s == prev {
			break
		}
	}
	return s
}

// leetNormalize applique uniquement le leet + minuscules (sans supprimer séparateurs).
func leetNormalize(s string) string {
	s = removeDiacritics(s)
	s = strings.ToLower(s)
	s = leetReplacer.Replace(s)
	return s
}

// removeDiacritics décompose d'abord en NFD (ó → o + combining acute),
// puis supprime les marques combinantes (catégorie Unicode Mn).
// Cela gère à la fois les caractères précomposés (ó, ñ, ü…)
// et les caractères déjà décomposés.
func removeDiacritics(s string) string {
	// NFD : décompose les caractères précomposés en lettre de base + marque
	s = norm.NFD.String(s)
	var b strings.Builder
	for _, r := range s {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ─── Patterns bruts (contournement par remplacement de lettre) ──────────────

// replChar correspond aux caractères typiquement utilisés pour REMPLACER une lettre
// (*, ., -, _, ~, ^, @, $) plutôt que comme séparateurs.
// On exclut volontairement les chiffres (gérés par la table leet).
const replChar = `[*._\-~^@$#!]`

// buildRawPatterns retourne des regex appliquées sur le texte leet-normalisé
// (sans suppression de séparateurs). Elles détectent les cas où un symbole
// REMPLACE une lettre entière : sh*t, f*ck, p.rn, n*gre, etc.
//
// Différence avec les wordPatterns :
//   - wordPatterns  → texte entièrement normalisé (séparateurs supprimés)
//   - rawPatterns   → texte après leet uniquement (séparateurs conservés)
//     Permet de voir que le `*` n'est PAS un séparateur mais bien un remplacement.
func buildRawPatterns() []*regexp.Regexp {
	raw := []string{
		// ── Français ──────────────────────────────────────────────────
		`(?i)n` + replChar + `gre`,              // n*gre  (e remplacé)
		`(?i)ne` + replChar + `gre`,             // ne*gre (milieu)
		`(?i)p` + replChar + `te`,               // p*te
		`(?i)enc` + replChar + `le`,             // enc*le (enculé)
		`(?i)b` + replChar + `ite`,              // b*ite
		`(?i)f` + replChar + `utre`,             // f*utre
		`(?i)niq` + replChar + `e`,              // niq*e (niquer)
		// ── English ───────────────────────────────────────────────────
		`(?i)f` + replChar + `ck`,               // f*ck (u remplacé)
		`(?i)sh` + replChar + `t`,               // sh*t (i remplacé)
		`(?i)c` + replChar + `nt`,               // c*nt (u remplacé)
		`(?i)b` + replChar + `tch`,              // b*tch (i remplacé)
		`(?i)d` + replChar + `ck`,               // d*ck (i remplacé)
		`(?i)c` + replChar + `ck`,               // c*ck (o remplacé)
		`(?i)p` + replChar + `rn`,               // p*rn / p.rn (o remplacé)
		`(?i)p` + replChar + `ssy`,              // p*ssy (u remplacé)
		`(?i)n` + replChar + `gger`,             // n*gger (i remplacé)
		`(?i)n` + replChar + `gga`,              // n*gga
		`(?i)r` + replChar + `pe`,               // r*pe (a remplacé)
		`(?i)ass` + replChar + `le`,             // ass*le / a**hole
		`(?i)wh` + replChar + `re`,              // wh*re (o remplacé)
		`(?i)sl` + replChar + `t`,               // sl*t (u remplacé)
		// ── Allemand ──────────────────────────────────────────────────
		`(?i)sch` + replChar + `ße`,             // sch*ße
		`(?i)sch` + replChar + `sse`,            // sch*sse
		`(?i)f` + replChar + `tze`,              // f*tze
		`(?i)w` + replChar + `chser`,            // w*chser
		`(?i)h` + replChar + `rensohn`,          // h*rensohn
		`(?i)n` + replChar + `ger`,              // N*ger (DE)
		`(?i)arschl` + replChar + `ch`,          // arschl*ch
		// ── Italien ───────────────────────────────────────────────────
		`(?i)c` + replChar + `zzo`,              // c*zzo (a remplacé)
		`(?i)v` + replChar + `ffanculo`,         // v*ffanculo
		`(?i)str` + replChar + `nzo`,            // str*nzo
		`(?i)putt` + replChar + `na`,            // putt*na (a remplacé)
		`(?i)f` + replChar + `ga`,               // f*ga (i remplacé)
		`(?i)cogli` + replChar + `ne`,           // cogli*ne
		// ── Espagnol ──────────────────────────────────────────────────
		`(?i)p` + replChar + `ta`,               // p*ta (u remplacé)
		`(?i)m` + replChar + `erda`,             // m*erda (i remplacé)
		`(?i)c` + replChar + `ño`,               // c*ño
		`(?i)c` + replChar + `no`,               // c*no (sans tilde)
		`(?i)j` + replChar + `der`,              // j*der
		`(?i)mar` + replChar + `con`,            // mar*con (maricón)
		`(?i)g` + replChar + `lipollas`,         // g*lipollas
	}

	compiled := make([]*regexp.Regexp, 0, len(raw))
	for _, r := range raw {
		compiled = append(compiled, regexp.MustCompile(r))
	}
	return compiled
}

// ─── Liste de mots ───────────────────────────────────────────────────────────

func buildWordList() []string {
	return []string{
		// =================================================================
		// FRANÇAIS (FR)
		// =================================================================
		// Insultes générales
		"putain", "pute", "fils de pute", "fdp", "ntm",
		"merde", "connard", "connasse", "con",
		"salope", "salopard", "saloperie",
		"enculer", "enculé", "enculée",
		"foutre", "va te faire foutre",
		"niquer", "nique", "niqué",
		"baise", "baiser",
		"enfoiré", "ordure", "trouduc",
		"tete de noeud", "branleur", "branlette", "branler",
		"chieur", "chieuse", "chiasse",
		"emmerdeur", "emmerde",
		"ta gueule",
		// Termes sexuels explicites
		"bite", "queue", "zob",
		"chatte", "foufoune",
		"cul", "anus",
		"couilles", "couille",
		"seins nus",
		"sucer", "sodomiser",
		"bander", "jouir",
		"branlette",
		// Slurs racistes / discriminatoires
		"negre", "negro", "negritude",
		"bougnoule",
		"raton",
		"youpin", "youpine",
		"bamboula",
		"bicot",
		"melon",
		"feuj",
		"bounty",
		"sale arabe", "sale noir", "sale blanc",
		// Orientation sexuelle (usages offensants)
		"pd", "pede", "tapette", "fiotte", "gouine",
		// CSAM / pédophilie
		"pedophile", "pedo", "pedoporn",
		"inceste",
		// Menaces / violences
		"viol", "violer", "violeur",
		"tuer", "mort",

		// =================================================================
		// ENGLISH (EN)
		// =================================================================
		// General insults
		"fuck", "fucker", "fucking", "fuckwit", "fuckhead",
		"shit", "shithead", "shitstain",
		"bitch", "son of a bitch",
		"asshole", "ass",
		"bastard",
		"motherfucker",
		"bullshit",
		"prick",
		"twat",
		"wanker", "wank",
		"douchebag",
		"dickhead", "dickwad",
		"dumbass", "jackass",
		"scumbag",
		"dipshit",
		"asshat",
		// Sexual terms
		"cunt",
		"dick", "cock",
		"pussy",
		"whore", "slut",
		"blowjob", "handjob",
		"porn", "xxx",
		"cum", "jizz",
		"tits", "boobs",
		// Racial / identity slurs
		"nigger", "nigga", "niga",
		"kike",
		"spic",
		"chink",
		"gook",
		"wetback",
		"towelhead", "raghead", "sandnigger",
		"zipperhead",
		"spook",
		"coon", "coons",
		"cracker",
		"beaner",
		"heeb", "hymie",
		"honky",
		// Sexual orientation / gender slurs
		"faggot", "fag",
		"dyke",
		"tranny", "trannies",
		// Ableist slurs
		"retard", "retarded",
		"spaz",
		// CSAM / predatory
		"pedophile", "pedophilia", "pedo", "nonce", "groomer",
		"rape", "rapist",

		// =================================================================
		// ALLEMAND (DE)
		// =================================================================
		// Insultes générales
		"scheiße", "scheisse", "scheis",
		"arschloch", "arsch",
		"wichser", "wichsen", "wix",
		"hurensohn", "hurenbock",
		"ficken", "fick",
		"verpiss dich", "halt die fresse", "leck mich am arsch",
		"vollidiot", "volldepp", "vollpfosten",
		"dummkopf", "blodmann", "trottel",
		"dreckstuck", "dreckkerl", "miststuck",
		"scheisskopf",
		"penner",
		"sau",
		// Termes sexuels
		"fotze",
		"schlampe",
		"nutte",
		"schwanz",
		"hure",
		"pisser",
		// Slurs raciaux / discriminatoires
		"neger",
		"kanake",
		"zigeuner",
		"judenschwein",
		"schlitzeuge",
		// Orientation sexuelle (usages offensants)
		"schwuchtel", "schwuler",
		"tunte",
		// CSAM / violences
		"kinderschander",
		"vergewaltiger",
		"vergewaltigen",

		// =================================================================
		// ITALIEN (IT)
		// =================================================================
		// Insultes générales
		"cazzo", "cazzate",
		"vaffanculo", "vaffan", "fanculo", "affanculo",
		"stronzo", "stronza",
		"bastardo", "bastarda",
		"figlio di puttana", "figlio di troia",
		"pezzo di merda",
		"rompicoglioni",
		"testa di cazzo",
		"che palle",
		"maledetto", "maledetta",
		"idiota",
		"imbecille",
		"cornuto", "cornuta",
		// Termes sexuels
		"puttana", "buttana",
		"merda",
		"coglione", "coglioni",
		"figa", "fica",
		"culo",
		"minchia",
		"scopare",
		"pompino",
		"mignotta", "baldracca",
		"fottiti",
		"troia",
		// Slurs raciaux / discriminatoires
		"negro", "negra",
		"sporco negro", "sporco straniero",
		"extracomunitario",
		"terrone",
		// Orientation sexuelle (usages offensants)
		"finocchio",
		"recchione",
		// Religion (blasphèmes graves)
		"porco dio", "porco dios",
		// CSAM / violences
		"pedofilo", "pedofilia",
		"stupro", "stupratore",

		// =================================================================
		// ESPAGNOL (ES)
		// =================================================================
		// Insultes générales
		"mierda",
		"hostia", "ostia",
		"joder",
		"cabron", "cabrones",
		"gilipollas",
		"hijo de puta", "hijoputa", "hdp",
		"putamadre",
		"imbecil",
		"idiota",
		"pedazo de mierda",
		"cretino",
		"capullo",
		"tonto del culo",
		"vete a la mierda",
		"que te jodan",
		"me cago en tu madre",
		// Termes sexuels
		"puta", "putita",
		"polla", "pollas",
		"verga",
		"coño", "cono",
		"culo",
		"follar",
		"coger",
		"chingar", "chinga", "chingadera", "chingado",
		"pendejo",
		"culero",
		"mamada", "mamadas",
		"chupapollas",
		"perra", "perras",
		"zorra", "zorras",
		"mamahuevo",
		"ojete",
		"concha",
		"pelotudo",
		"carajo",
		"pinche",
		"huevon", "weon",
		// Slurs raciaux / discriminatoires
		"negro", "negra",
		"sudaca",
		"marron",
		"indio de mierda",
		"moro", "moros",
		"sudado",
		// Orientation sexuelle (usages offensants)
		"maricon", "maricones",
		"joton",
		// CSAM / violences
		"pedofilo", "pedofilia",
		"violacion", "violador",
	}
}
