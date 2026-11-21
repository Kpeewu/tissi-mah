package filter_test

import (
	"testing"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/filter"
)

func TestFilterBlocks(t *testing.T) {
	f := filter.NewTextFilter()

	cases := []struct {
		text    string
		desc    string
		blocked bool
	}{
		// Leet-speak (chiffres remplaçant des lettres)
		{"p0rn", "EN leet 0→o", true},
		{"sh!t", "EN leet !→i", true},
		{"b1tch", "EN leet 1→i", true},
		{"d1ck", "EN leet 1→i", true},
		{"@$$hole", "EN leet @→a $→s", true},
		{"n3gr3", "FR leet 3→e (negre)", true},
		{"c4zzo", "IT leet 4→a", true},
		{"w1chser", "DE leet 1→i", true},
		{"m1erda", "ES leet 1→i", true},
		{"put@", "ES leet @→a", true},

		// Séparateurs entre lettres
		{"f.u.c.k", "EN points", true},
		{"f_u_c_k", "EN underscores", true},
		{"f-u-c-k", "EN tirets", true},
		{"c.u.n.t", "EN points", true},
		{"s.h.i.t", "EN points", true},

		// Remplacement d'une lettre par un symbole
		{"sh*t", "EN * remplace voyelle", true},
		{"b*tch", "EN * remplace voyelle", true},
		{"c*nt", "EN * remplace voyelle", true},
		{"n*gre", "FR * remplace voyelle", true},
		{"p.rn", "EN . remplace voyelle", true},
		{"f*ck", "EN * remplace voyelle", true},
		{"d*ck", "EN * remplace voyelle", true},

		// Directs
		{"putain", "FR direct", true},
		{"nigger", "EN direct", true},
		{"wichser", "DE direct", true},
		{"vaffanculo", "IT direct", true},
		{"gilipollas", "ES direct", true},
		{"cornuto", "IT direct", true},
		{"hurensohn", "DE direct", true},
		{"niquer", "FR direct", true},
		{"connard", "FR direct", true},
		{"coglione", "IT direct", true},
		{"maricón", "ES direct (avec accent)", true},

		// Faux positifs à ne PAS bloquer
		{"bonjour", "salutation", false},
		{"merci", "politesse", false},
		{"chauffeur", "profession", false},
		{"conducteur", "profession", false},
		{"covoiturage", "sujet app", false},
	}

	blocked := 0
	passed := 0
	fp := 0
	fn := 0

	for _, c := range cases {
		r := f.Analyze(c.text)
		isBlocked := r.Score >= 0.9
		if isBlocked {
			blocked++
		} else {
			passed++
		}
		if c.blocked && !isBlocked {
			fn++
			t.Errorf("FAUX_NEGATIF %-20q (%s) → score %.2f, matched=%q", c.text, c.desc, r.Score, r.Matched)
		}
		if !c.blocked && isBlocked {
			fp++
			t.Errorf("FAUX_POSITIF %-20q (%s) → score %.2f, matched=%q", c.text, c.desc, r.Score, r.Matched)
		}
	}

	t.Logf("blocked=%d  passed=%d  faux_negatifs=%d  faux_positifs=%d", blocked, passed, fn, fp)
}
