package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/implementations"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// --- Factories de repos ---

func newUserDocReadRepo() repoInterfaces.UserDocumentRepositoryRead {
	return implementations.NewUserDocumentReadRepository(testPool, testLogger)
}

func newUserDocWriteRepo() repoInterfaces.UserDocumentRepositoryWrite {
	return implementations.NewUserDocumentWriteRepository(testPool, testLogger)
}

func newVehicleDocReadRepo() repoInterfaces.VehicleDocumentRepositoryRead {
	return implementations.NewVehicleDocumentReadRepository(testPool, testLogger)
}

func newVehicleDocWriteRepo() repoInterfaces.VehicleDocumentRepositoryWrite {
	return implementations.NewVehicleDocumentWriteRepository(testPool, testLogger)
}

func newReviewReadRepo() repoInterfaces.DocumentReviewRepositoryRead {
	return implementations.NewDocumentReviewReadRepository(testPool, testLogger)
}

func newReviewWriteRepo() repoInterfaces.DocumentReviewRepositoryWrite {
	return implementations.NewDocumentReviewWriteRepository(testPool, testLogger)
}

// --- Nettoyage ---

func cleanTables(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	_, err := testPool.Exec(ctx, `DELETE FROM document_reviews`)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `DELETE FROM user_documents`)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `DELETE FROM vehicle_documents`)
	require.NoError(t, err)
}

// --- Fixtures : UserDocument ---

func newUserDoc(userID, docType string) *domain.UserDocument {
	now := time.Now().UTC().Truncate(time.Millisecond)
	issued := now.AddDate(-1, 0, 0)
	expiry := now.AddDate(5, 0, 0)
	return &domain.UserDocument{
		DocumentID:     uuid.New().String(),
		UserID:         userID,
		DocumentName:   docType + "_doc",
		DocumentType:   docType,
		DocumentKey:    docType + "/" + userID + "/doc.jpg",
		FileSizeBytes:  1024,
		MimeType:       "image/jpeg",
		DocumentNumber: "TEST-" + docType,
		IssuedAt:       &issued,
		ExpireAt:       &expiry,
		IssuingCountry: "TG",
		Status:         "pending",
		IsCurrent:      true,
		UploadedAt:     now,
		UpdatedAt:      now,
	}
}

func insertUserDoc(t *testing.T, doc *domain.UserDocument) {
	t.Helper()
	repo := newUserDocWriteRepo()
	_, err := repo.Create(context.Background(), doc)
	require.NoError(t, err)
}

// --- Fixtures : VehicleDocument ---

func newVehicleDoc(vehicleID, docType string) *domain.VehicleDocument {
	now := time.Now().UTC().Truncate(time.Millisecond)
	issued := now.AddDate(-1, 0, 0)
	expiry := now.AddDate(3, 0, 0)
	return &domain.VehicleDocument{
		DocumentID: uuid.New().String(),
		VehicleID:  vehicleID,
		// UserID par défaut dérivé du vehicleID : les tests qui ont besoin d'un
		// utilisateur spécifique peuvent écraser ce champ après l'appel.
		// Indispensable depuis migration 000008 où document_reviews.user_id est NOT NULL.
		UserID:           "owner-" + vehicleID,
		DocumentName:     docType + "_doc",
		DocumentType:     docType,
		DocumentKey:      docType + "/" + vehicleID + "/doc.jpg",
		FileSizeBytes:    2048,
		MimeType:         "image/jpeg",
		DocumentNumber:   "VEH-" + docType,
		IssuedAt:         &issued,
		ExpireAt:         &expiry,
		IssuingAuthority: "test-authority",
		Status:           "pending",
		IsCurrent:        true,
		UploadedAt:       now,
		UpdatedAt:        now,
	}
}

func insertVehicleDoc(t *testing.T, doc *domain.VehicleDocument) {
	t.Helper()
	repo := newVehicleDocWriteRepo()
	_, err := repo.Create(context.Background(), doc)
	require.NoError(t, err)
}

// --- Fixtures : DocumentReview ---

// newReviewForUserDoc construit une review attachée à un user document.
// UserID et DocumentType sont dénormalisés depuis migration 000008 (NOT NULL),
// donc on prend le doc complet pour les récupérer.
func newReviewForUserDoc(doc *domain.UserDocument) *domain.DocumentReview {
	now := time.Now().UTC().Truncate(time.Millisecond)
	docID := doc.DocumentID
	return &domain.DocumentReview{
		ReviewID:       uuid.New().String(),
		UserID:         doc.UserID,
		DocumentType:   doc.DocumentType,
		UserDocumentID: &docID,
		Status:         "pending",
		Decision:       "approved",
		ReviewedBy:     "admin-1",
		ReviewType:     "manual",
		AttemptNumber:  1,
		ReviewedAt:     &now,
		UpdatedAt:      now,
	}
}

// newReviewForVehicleDoc construit une review attachée à un vehicle document.
// Idem : UserID (= UserID du véhicule, migration 000005) + DocumentType dénormalisés.
func newReviewForVehicleDoc(doc *domain.VehicleDocument) *domain.DocumentReview {
	now := time.Now().UTC().Truncate(time.Millisecond)
	docID := doc.DocumentID
	return &domain.DocumentReview{
		ReviewID:          uuid.New().String(),
		UserID:            doc.UserID,
		DocumentType:      doc.DocumentType,
		VehicleDocumentID: &docID,
		Status:            "completed",
		Decision:          "rejected",
		ReasonRejection:   "document_expired",
		ReviewedBy:        "bot-1",
		ReviewType:        "automatic",
		AttemptNumber:     1,
		ReviewedAt:        &now,
		UpdatedAt:         now,
	}
}

func insertReview(t *testing.T, review *domain.DocumentReview) {
	t.Helper()
	repo := newReviewWriteRepo()
	_, err := repo.Create(context.Background(), review)
	require.NoError(t, err)
}
