package domain_test

import (
	"testing"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

// approvedDoc construit un document utilisateur courant approuvé.
func approvedDoc(documentType string) *domain.VerificationDocument {
	return &domain.VerificationDocument{DocumentType: documentType, Status: "approved"}
}

// approvedVehicleDoc construit un document véhicule courant approuvé.
func approvedVehicleDoc(documentType, vehicleID string) *domain.VerificationDocument {
	return &domain.VerificationDocument{DocumentType: documentType, Status: "approved", VehicleID: vehicleID}
}

func TestComputeProfileVerification(t *testing.T) {
	fullVehicle := []*domain.VerificationDocument{
		approvedVehicleDoc("insurance", "veh1"),
		approvedVehicleDoc("registrationCard", "veh1"),
	}
	fullLicence := []*domain.VerificationDocument{
		approvedDoc("driverLicenceFront"),
		approvedDoc("driverLicenceBack"),
	}

	tests := []struct {
		name         string
		userDocs     []*domain.VerificationDocument
		vehicleDocs  []*domain.VerificationDocument
		wantIdentity bool // == IsPassengerProfileVerified
		wantDriver   bool
	}{
		{
			name:         "aucun document",
			wantIdentity: false,
			wantDriver:   false,
		},
		{
			name:         "pièce approuvée sans selfie → non vérifié (le selfie est obligatoire)",
			userDocs:     []*domain.VerificationDocument{approvedDoc("passport")},
			wantIdentity: false,
			wantDriver:   false,
		},
		{
			name:         "selfie seul sans pièce → non vérifié",
			userDocs:     []*domain.VerificationDocument{approvedDoc("selfie")},
			wantIdentity: false,
			wantDriver:   false,
		},
		{
			name:         "selfie + passeport approuvés → passager vérifié",
			userDocs:     []*domain.VerificationDocument{approvedDoc("selfie"), approvedDoc("passport")},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name: "selfie + CNI recto+verso → passager vérifié",
			userDocs: []*domain.VerificationDocument{
				approvedDoc("selfie"), approvedDoc("idCardFront"), approvedDoc("idCardBack"),
			},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name: "CNI recto seul (verso manquant) + selfie → non vérifié",
			userDocs: []*domain.VerificationDocument{
				approvedDoc("selfie"), approvedDoc("idCardFront"),
			},
			wantIdentity: false,
			wantDriver:   false,
		},
		{
			name:         "selfie + permis recto+verso, sans véhicule → passager vérifié (double rôle), pas conducteur",
			userDocs:     append([]*domain.VerificationDocument{approvedDoc("selfie")}, fullLicence...),
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name:         "selfie + permis + véhicule entièrement validé → passager ET conducteur",
			userDocs:     append([]*domain.VerificationDocument{approvedDoc("selfie")}, fullLicence...),
			vehicleDocs:  fullVehicle,
			wantIdentity: true,
			wantDriver:   true,
		},
		{
			name:     "permis + véhicule validé mais selfie manquant → rien",
			userDocs: fullLicence,
			vehicleDocs: []*domain.VerificationDocument{
				approvedVehicleDoc("insurance", "veh1"),
				approvedVehicleDoc("registrationCard", "veh1"),
			},
			wantIdentity: false,
			wantDriver:   false,
		},
		{
			name:     "véhicule incomplet (assurance seule) → conducteur non vérifié",
			userDocs: append([]*domain.VerificationDocument{approvedDoc("selfie")}, fullLicence...),
			vehicleDocs: []*domain.VerificationDocument{
				approvedVehicleDoc("insurance", "veh1"),
			},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name:     "documents répartis sur 2 véhicules (aucun complet) → conducteur non vérifié",
			userDocs: append([]*domain.VerificationDocument{approvedDoc("selfie")}, fullLicence...),
			vehicleDocs: []*domain.VerificationDocument{
				approvedVehicleDoc("insurance", "veh1"),
				approvedVehicleDoc("registrationCard", "veh2"),
			},
			wantIdentity: true,
			wantDriver:   false,
		},
		{
			name:     "2 véhicules dont UN entièrement validé → conducteur vérifié",
			userDocs: append([]*domain.VerificationDocument{approvedDoc("selfie")}, fullLicence...),
			vehicleDocs: []*domain.VerificationDocument{
				approvedVehicleDoc("insurance", "veh1"),
				approvedVehicleDoc("registrationCard", "veh1"),
				approvedVehicleDoc("insurance", "veh2"),
				{DocumentType: "registrationCard", Status: "rejected", VehicleID: "veh2"},
			},
			wantIdentity: true,
			wantDriver:   true,
		},
		{
			name: "selfie remplacé en attente (pending) → identité retombe non vérifiée",
			userDocs: []*domain.VerificationDocument{
				{DocumentType: "selfie", Status: "pending"},
				approvedDoc("passport"),
			},
			wantIdentity: false,
			wantDriver:   false,
		},
		{
			name: "documents rejetés ne comptent pas",
			userDocs: []*domain.VerificationDocument{
				{DocumentType: "selfie", Status: "rejected"},
				{DocumentType: "idCardFront", Status: "rejected"},
				{DocumentType: "idCardBack", Status: "rejected"},
			},
			wantIdentity: false,
			wantDriver:   false,
		},
		{
			name: "idCard + selfie mais permis manquant, véhicule validé → passager seulement",
			userDocs: []*domain.VerificationDocument{
				approvedDoc("selfie"), approvedDoc("idCardFront"), approvedDoc("idCardBack"),
			},
			vehicleDocs:  fullVehicle,
			wantIdentity: true,
			wantDriver:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, driver := domain.ComputeProfileVerification(tt.userDocs, tt.vehicleDocs)
			assert.Equal(t, tt.wantIdentity, identity, "identityVerified (passager)")
			assert.Equal(t, tt.wantDriver, driver, "driverVerified")
		})
	}
}

func TestComputeVehicleVerification(t *testing.T) {
	out := domain.ComputeVehicleVerification([]*domain.VerificationDocument{
		approvedVehicleDoc("insurance", "veh1"),
		approvedVehicleDoc("registrationCard", "veh1"),
		approvedVehicleDoc("insurance", "veh2"),
		{DocumentType: "registrationCard", Status: "pending", VehicleID: "veh3"},
	})
	assert.True(t, out["veh1"], "assurance + carte grise approuvées")
	assert.False(t, out["veh2"], "carte grise manquante")
	assert.False(t, out["veh3"], "aucun document approuvé")
	assert.Len(t, out, 3, "tous les véhicules avec au moins un document sont listés (pour dé-vérification)")
}

func TestDocumentCategories(t *testing.T) {
	assert.Equal(t, []string{"passenger", "driver"}, domain.DocumentCategories("driverLicenceFront", "user"),
		"le permis a un double rôle explicite : identité ET conduite")
	assert.Equal(t, []string{"passenger", "driver"}, domain.DocumentCategories("driverLicenceBack", "user"))
	assert.Equal(t, []string{"passenger"}, domain.DocumentCategories("selfie", "user"))
	assert.Equal(t, []string{"passenger"}, domain.DocumentCategories("idCardFront", "user"))
	assert.Equal(t, []string{"passenger"}, domain.DocumentCategories("passport", "user"))
	assert.Equal(t, []string{"driver"}, domain.DocumentCategories("insurance", "vehicle"))
	assert.Nil(t, domain.DocumentCategories("profilePicture", "user"), "profilePicture exclu des files de validation")
}
