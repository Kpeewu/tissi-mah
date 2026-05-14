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
	return &domain.UserDocument{
		DocumentID:    uuid.New().String(),
		UserID:        userID,
		DocumentName:  docType + "_doc",
		DocumentType:  docType,
		DocumentKey:   docType + "/" + userID + "/doc.jpg",
		FileSizeBytes: 1024,
		MimeType:      "image/jpeg",
		Status:        "pending",
		IsCurrent:     true,
		UploadedAt:    now,
		UpdatedAt:     now,
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
	return &domain.VehicleDocument{
		DocumentID:    uuid.New().String(),
		VehicleID:     vehicleID,
		DocumentName:  docType + "_doc",
		DocumentType:  docType,
		DocumentKey:   docType + "/" + vehicleID + "/doc.jpg",
		FileSizeBytes: 2048,
		MimeType:      "image/jpeg",
		Status:        "pending",
		IsCurrent:     true,
		UploadedAt:    now,
		UpdatedAt:     now,
	}
}

func insertVehicleDoc(t *testing.T, doc *domain.VehicleDocument) {
	t.Helper()
	repo := newVehicleDocWriteRepo()
	_, err := repo.Create(context.Background(), doc)
	require.NoError(t, err)
}

// --- Fixtures : DocumentReview ---

func newReviewForUserDoc(userDocID string) *domain.DocumentReview {
	now := time.Now().UTC().Truncate(time.Millisecond)
	return &domain.DocumentReview{
		ReviewID:       uuid.New().String(),
		UserDocumentID: &userDocID,
		Status:         "pending",
		Decision:       "approved",
		ReviewedBy:     "admin-1",
		ReviewType:     "manual",
		AttemptNumber:  1,
		ReviewedAt:     now,
		UpdatedAt:      now,
	}
}

func newReviewForVehicleDoc(vehicleDocID string) *domain.DocumentReview {
	now := time.Now().UTC().Truncate(time.Millisecond)
	return &domain.DocumentReview{
		ReviewID:          uuid.New().String(),
		VehicleDocumentID: &vehicleDocID,
		Status:            "completed",
		Decision:          "rejected",
		ReasonRejection:   "document_expired",
		ReviewedBy:        "bot-1",
		ReviewType:        "automatic",
		AttemptNumber:     1,
		ReviewedAt:        now,
		UpdatedAt:         now,
	}
}

func insertReview(t *testing.T, review *domain.DocumentReview) {
	t.Helper()
	repo := newReviewWriteRepo()
	_, err := repo.Create(context.Background(), review)
	require.NoError(t, err)
}
