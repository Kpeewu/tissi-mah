package service

import "time"

// PaymentWindow définit une fenêtre horaire quotidienne pendant laquelle les
// workers automatiques de paiement (payout chauffeurs, refund passagers) sont
// autorisés à exécuter leurs batches. Hors fenêtre, le tick du worker passe
// sans rien faire — la cadence du ticker reste inchangée.
//
// Si Enabled=false, la fenêtre est ignorée (workers tournent 24/7) — utile
// en dev/local pour ne pas bloquer les tests d'intégration.
//
// La fenêtre est [StartHour, EndHour) dans Location. Supporte le wrap
// minuit (ex: StartHour=22, EndHour=3 → 22h→23h59 puis 0h→3h).
type PaymentWindow struct {
	Enabled   bool
	Location  *time.Location
	StartHour int // 0-23
	EndHour   int // 1-24, exclusif
}

// IsOpen retourne true si t (converti en Location) tombe dans la fenêtre.
func (w PaymentWindow) IsOpen(t time.Time) bool {
	if !w.Enabled {
		return true
	}
	if w.Location == nil {
		return true
	}
	h := t.In(w.Location).Hour()
	if w.StartHour == w.EndHour {
		return false
	}
	if w.StartHour < w.EndHour {
		return h >= w.StartHour && h < w.EndHour
	}
	// Wrap minuit
	return h >= w.StartHour || h < w.EndHour
}
