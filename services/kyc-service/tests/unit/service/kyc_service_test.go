package service_test

import (
	"testing"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service/interfaces"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/tests/mocks"
)

// --- Helpers ---

// newTestService crée un triplet (fileClient, personaClient) mockés
// et le service instancié à partir de ces mocks.
func newTestService() (*mocks.MockFileServiceClient, *mocks.MockPersonaClient, serviceInterfaces.KYCService) {
	mockFileClient := new(mocks.MockFileServiceClient)
	mockPersonaClient := new(mocks.MockPersonaClient)
	svc := service.NewKYCService(mockFileClient, mockPersonaClient, zap.NewNop())
	return mockFileClient, mockPersonaClient, svc
}

// =============================================================================
// CreateInquiry
// =============================================================================

func TestCreateInquiry(t *testing.T) {
	// Les tests seront ajoutés lors de l'implémentation de CreateInquiry
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// GetInquiry
// =============================================================================

func TestGetInquiry(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// GetKYCStatus
// =============================================================================

func TestGetKYCStatus(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// ResumeInquiry
// =============================================================================

func TestResumeInquiry(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// ProcessWebhook
// =============================================================================

func TestProcessWebhook(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// GetAdminReviews
// =============================================================================

func TestGetAdminReviews(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// GetAdminReview
// =============================================================================

func TestGetAdminReview(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// OverrideReview
// =============================================================================

func TestOverrideReview(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}
