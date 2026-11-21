package domain_test

import (
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 9, 22, 15, 0, 0, 0, time.UTC)

func at(minutes int) *time.Time {
	t := t0.Add(time.Duration(minutes) * time.Minute)
	return &t
}

const statusPending = "pending"

func review(id, logicalType, decision string, reviewedAt *time.Time, attempt int32) *domain.Review {
	status := "completed"
	if decision == "" {
		status = statusPending
	}
	return &domain.Review{
		ReviewID: id, LogicalDocumentType: logicalType, DocumentType: logicalType,
		Status: status, Decision: decision, ReviewedAt: reviewedAt,
		AttemptNumber: attempt, CreatedAt: t0,
	}
}

func TestLatestActionableRejection_rejetSimple(t *testing.T) {
	got := domain.LatestActionableRejection([]*domain.Review{
		review("r1", "selfie", "rejected", at(10), 1),
	})
	require.NotNil(t, got)
	assert.Equal(t, "r1", got.ReviewID)
}

// Cas constaté sur VPS Dev le 22/09 : le selfie rejeté puis ré-évalué « approuvé »
// restait signalé comme rejeté, car OverrideReview crée une nouvelle review et laisse
// l'ancienne intacte.
func TestLatestActionableRejection_rejetCorrigeParReevaluation(t *testing.T) {
	got := domain.LatestActionableRejection([]*domain.Review{
		review("r1", "selfie", "rejected", at(10), 1),
		review("r2", "selfie", "approved", at(20), 2),
	})
	assert.Nil(t, got)
}

func TestLatestActionableRejection_rejetCorrigeParNouvelEnvoi(t *testing.T) {
	pending := review("r2", "selfie", "", nil, 2)
	pending.CreatedAt = t0.Add(30 * time.Minute)
	got := domain.LatestActionableRejection([]*domain.Review{
		review("r1", "selfie", "rejected", at(10), 1),
		pending,
	})
	assert.Nil(t, got, "un document renvoyé et en attente ne doit plus être signalé comme rejeté")
}

func TestLatestActionableRejection_resoumissionDemandee(t *testing.T) {
	got := domain.LatestActionableRejection([]*domain.Review{
		review("r1", "driverLicence", "resubmission", at(10), 1),
	})
	require.NotNil(t, got)
	assert.Equal(t, "r1", got.ReviewID)
}

// Le scénario complet du 22/09 : selfie ré-évalué approuvé, permis renvoyé pour
// resoumission, documents véhicule approuvés — seul le permis demande une action.
func TestLatestActionableRejection_scenarioDu22Septembre(t *testing.T) {
	got := domain.LatestActionableRejection([]*domain.Review{
		review("selfie-1", "selfie", "rejected", at(41), 1),
		review("permis-1", "driverLicence", "rejected", at(42), 1),
		review("selfie-2", "selfie", "approved", at(60), 2),
		review("permis-2", "driverLicence", "resubmission", at(61), 2),
		review("insurance-1", "insurance", "approved", at(62), 1),
		review("registration-1", "registrationCard", "approved", at(63), 1),
	})
	require.NotNil(t, got)
	assert.Equal(t, "permis-2", got.ReviewID)
}

func TestLatestActionableRejection_rectoEtVersoSontLeMemeDocument(t *testing.T) {
	front := review("r1", "", "rejected", at(10), 1)
	front.DocumentType = "idCardFront"
	back := review("r2", "", "approved", at(20), 2)
	back.DocumentType = "idCardBack"
	assert.Nil(t, domain.LatestActionableRejection([]*domain.Review{front, back}))
}

func TestLatestActionableRejection_plusRecentDesRejetsEnCours(t *testing.T) {
	got := domain.LatestActionableRejection([]*domain.Review{
		review("r1", "selfie", "rejected", at(10), 1),
		review("r2", "insurance", "rejected", at(30), 1),
	})
	require.NotNil(t, got)
	assert.Equal(t, "r2", got.ReviewID)
}

func TestLatestActionableRejection_aucuneReview(t *testing.T) {
	assert.Nil(t, domain.LatestActionableRejection(nil))
}
