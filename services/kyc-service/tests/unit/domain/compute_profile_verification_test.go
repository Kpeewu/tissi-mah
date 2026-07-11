package domain_test

import (
	"testing"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

// approvedReview construit une review approuvée du type physique donné.
func approvedReview(documentType string) *domain.Review {
	return &domain.Review{
		DocumentType: documentType,
		Status:       "completed",
		Decision:     "approved",
	}
}

func TestComputeProfileVerification(t *testing.T) {
	tests := []struct {
		name         string
		reviews      []*domain.Review
		wantIdentity bool // == IsPassengerProfileVerified
		wantDriver   bool
	}{
		{
			name:         "aucune review",
			reviews:      nil,
			wantIdentity: false,
			wantDriver:   false,
		},
		{
			name:         "carte d'identité approuvée seule → passager, pas conducteur",
			reviews:      []*domain.Review{approvedReview("idCardFront")},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name:         "passeport approuvé seul → passager, pas conducteur",
			reviews:      []*domain.Review{approvedReview("passport")},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name:         "permis approuvé seul vaut pièce d'identité → passager, pas conducteur",
			reviews:      []*domain.Review{approvedReview("driverLicenceFront")},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name: "permis + assurance + carte grise → passager ET conducteur (sans idCard/passport)",
			reviews: []*domain.Review{
				approvedReview("driverLicenceBack"), // le verso mappe aussi vers driverLicence
				approvedReview("insurance"),
				approvedReview("registrationCard"),
			},
			wantIdentity: true,
			wantDriver:   true,
		},
		{
			name: "idCard + permis + assurance + carte grise → passager ET conducteur",
			reviews: []*domain.Review{
				approvedReview("idCardFront"),
				approvedReview("driverLicenceFront"),
				approvedReview("insurance"),
				approvedReview("registrationCard"),
			},
			wantIdentity: true,
			wantDriver:   true,
		},
		{
			name: "permis + assurance mais carte grise manquante → conducteur non vérifié",
			reviews: []*domain.Review{
				approvedReview("driverLicenceFront"),
				approvedReview("insurance"),
			},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name: "idCard + assurance + carte grise mais permis manquant → conducteur non vérifié",
			reviews: []*domain.Review{
				approvedReview("idCardFront"),
				approvedReview("insurance"),
				approvedReview("registrationCard"),
			},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name: "reviews rejetées ne comptent pas",
			reviews: []*domain.Review{
				{DocumentType: "idCardFront", Status: "completed", Decision: "rejected"},
				{DocumentType: "driverLicenceFront", Status: "completed", Decision: "rejected"},
			},
			wantIdentity: false,
			wantDriver:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, driver := domain.ComputeProfileVerification(tt.reviews)
			assert.Equal(t, tt.wantIdentity, identity, "identityVerified (passager)")
			assert.Equal(t, tt.wantDriver, driver, "driverVerified")
		})
	}
}
