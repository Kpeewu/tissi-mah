package domain

import (
	"strings"
	"time"
)

// Décisions qui demandent à l'utilisateur de renvoyer un document.
const (
	decisionRejected     = "rejected"
	decisionResubmission = "resubmission"
)

// actionableDecisions sont les décisions qui demandent à l'utilisateur de renvoyer
// un document : un rejet, ou une demande explicite de resoumission.
var actionableDecisions = map[string]bool{
	decisionRejected:     true,
	decisionResubmission: true,
}

// LatestActionableRejection retourne la review la plus récente qui demande encore une
// action à l'utilisateur, ou nil.
//
// Une review n'est retenue que si elle est la DERNIÈRE de son document logique. Sans
// ce filtre, un rejet restait signalé indéfiniment après correction :
//   - une ré-évaluation (OverrideReview) crée une nouvelle review approuvée et laisse
//     l'ancienne intacte, avec sa décision « rejected » ;
//   - un nouvel envoi crée une review « pending » plus récente.
//
// Les décisions « resubmission » sont traitées comme des rejets : elles demandent elles
// aussi un nouvel envoi, et n'étaient jusqu'ici remontées ni comme rejet ni comme attente.
func LatestActionableRejection(reviews []*Review) *Review {
	latestByDocument := make(map[string]*Review)
	for _, r := range reviews {
		key := logicalDocumentKey(r)
		if current, ok := latestByDocument[key]; !ok || isMoreRecent(r, current) {
			latestByDocument[key] = r
		}
	}

	var latest *Review
	for _, r := range latestByDocument {
		if !actionableDecisions[r.Decision] || r.ReviewedAt == nil {
			continue
		}
		if latest == nil || r.ReviewedAt.After(*latest.ReviewedAt) {
			latest = r
		}
	}
	return latest
}

// logicalDocumentKey identifie le document logique d'une review : recto et verso d'une
// même pièce partagent la même clé.
func logicalDocumentKey(r *Review) string {
	t := r.LogicalDocumentType
	if t == "" {
		t = r.DocumentType
	}
	t = strings.ToLower(strings.TrimSpace(t))
	t = strings.TrimSuffix(t, "front")
	t = strings.TrimSuffix(t, "back")
	return t
}

// reviewTime est la date qui ordonne les reviews d'un même document : la décision si
// elle existe, sinon l'envoi, sinon la création.
func reviewTime(r *Review) time.Time {
	switch {
	case r.ReviewedAt != nil:
		return *r.ReviewedAt
	case r.SubmittedAt != nil:
		return *r.SubmittedAt
	default:
		return r.CreatedAt
	}
}

func isMoreRecent(a, b *Review) bool {
	ta, tb := reviewTime(a), reviewTime(b)
	if !ta.Equal(tb) {
		return ta.After(tb)
	}
	return a.AttemptNumber > b.AttemptNumber
}
