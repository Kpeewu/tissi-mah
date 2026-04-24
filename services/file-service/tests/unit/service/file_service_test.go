package service_test

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/file-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newService crée une instance du service avec tous les mocks.
func newService(
	userDocRead *mocks.MockUserDocumentRepositoryRead,
	userDocWrite *mocks.MockUserDocumentRepositoryWrite,
	vehicleDocRead *mocks.MockVehicleDocumentRepositoryRead,
	vehicleDocWrite *mocks.MockVehicleDocumentRepositoryWrite,
	reviewRead *mocks.MockDocumentReviewRepositoryRead,
	reviewWrite *mocks.MockDocumentReviewRepositoryWrite,
	storage *mocks.MockStorageClient,
) serviceInterfaces.FileService {
	return service.NewFileService(
		userDocRead,
		userDocWrite,
		vehicleDocRead,
		vehicleDocWrite,
		reviewRead,
		reviewWrite,
		storage,
		zap.NewNop(),
	)
}

// stubUserDoc retourne un document utilisateur minimal pour les tests.
func stubUserDoc(docID, userID, docType string) *domain.UserDocument {
	return &domain.UserDocument{
		DocumentID:   docID,
		UserID:       userID,
		DocumentName: "test_doc",
		DocumentType: docType,
		DocumentURL:  "https://storage.example.com/" + docType + "/" + userID + "/" + docID + ".jpg",
		MimeType:     "image/jpeg",
		Status:       "pending",
		IsCurrent:    true,
	}
}

// stubVehicleDoc retourne un document véhicule minimal pour les tests.
func stubVehicleDoc(docID, vehicleID, docType string) *domain.VehicleDocument {
	return &domain.VehicleDocument{
		DocumentID:   docID,
		VehicleID:    vehicleID,
		DocumentName: "test_vehicle_doc",
		DocumentType: docType,
		DocumentURL:  "https://storage.example.com/" + docType + "/" + vehicleID + "/" + docID + ".jpg",
		MimeType:     "image/jpeg",
		Status:       "pending",
		IsCurrent:    true,
	}
}

// fakeJPEG retourne un slice de bytes valide pour simuler un fichier JPEG (magic bytes).
func fakeJPEG() []byte {
	// JPEG magic bytes : FF D8 FF
	b := make([]byte, 512)
	b[0] = 0xFF
	b[1] = 0xD8
	b[2] = 0xFF
	return b
}

// =============================================================================
// UploadUserDocument
// =============================================================================

func TestFileService_UploadUserDocument(t *testing.T) {
	t.Run("should upload and return user document", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, "image/jpeg", int64(100)).
			Return("https://storage.example.com/idCardFront/user-1/doc.jpg", nil)
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "idCardFront").
			Return(nil, fileErrors.ErrorDocumentNotFound)
		userDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Return("doc-1", nil)

		svc := newService(userDocRead, userDocWrite, vehicleDocRead, vehicleDocWrite, reviewRead, reviewWrite, storage)

		result, err := svc.UploadUserDocument(context.Background(), serviceInterfaces.UploadUserDocumentInput{
			UserID:        "user-1",
			DocumentName:  "id_card",
			DocumentType:  "idCardFront",
			MimeType:      "image/jpeg",
			FileSizeBytes: 100,
			Data:          bytes.NewReader([]byte("fake")),
		})

		require.NoError(t, err)
		assert.NotEmpty(t, result.DocumentID)
		assert.Equal(t, "user-1", result.UserID)
		assert.Equal(t, "idCardFront", result.DocumentType)
		assert.Equal(t, "pending", result.Status)
		assert.True(t, result.IsCurrent)
		storage.AssertExpectations(t)
		userDocWrite.AssertExpectations(t)
	})

	t.Run("should mark previous document as replaced when one exists", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		existing := stubUserDoc("old-doc", "user-1", "idCardFront")
		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, "image/jpeg", int64(100)).
			Return("https://storage.example.com/idCardFront/user-1/new.jpg", nil)
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "idCardFront").
			Return(existing, nil)
		userDocWrite.On("MarkAsReplaced", mock.Anything, "old-doc", mock.AnythingOfType("string")).
			Return(nil)
		userDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Return("new-doc", nil)

		svc := newService(userDocRead, userDocWrite, vehicleDocRead, vehicleDocWrite, reviewRead, reviewWrite, storage)

		_, err := svc.UploadUserDocument(context.Background(), serviceInterfaces.UploadUserDocumentInput{
			UserID:        "user-1",
			DocumentType:  "idCardFront",
			MimeType:      "image/jpeg",
			FileSizeBytes: 100,
			Data:          bytes.NewReader([]byte("fake")),
		})

		require.NoError(t, err)
		userDocWrite.AssertCalled(t, "MarkAsReplaced", mock.Anything, "old-doc", mock.AnythingOfType("string"))
	})

	t.Run("should return ErrorInvalidDocumentType for unknown type", func(t *testing.T) {
		svc := newService(
			&mocks.MockUserDocumentRepositoryRead{},
			&mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{},
			&mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{},
			&mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{},
		)

		_, err := svc.UploadUserDocument(context.Background(), serviceInterfaces.UploadUserDocumentInput{
			UserID:        "user-1",
			DocumentType:  "invalid_type",
			MimeType:      "image/jpeg",
			FileSizeBytes: 100,
			Data:          bytes.NewReader([]byte("fake")),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})

	t.Run("should return ErrorInvalidMimeType for unsupported mime", func(t *testing.T) {
		svc := newService(
			&mocks.MockUserDocumentRepositoryRead{},
			&mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{},
			&mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{},
			&mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{},
		)

		_, err := svc.UploadUserDocument(context.Background(), serviceInterfaces.UploadUserDocumentInput{
			UserID:        "user-1",
			DocumentType:  "idCardFront",
			MimeType:      "video/mp4",
			FileSizeBytes: 100,
			Data:          bytes.NewReader([]byte("fake")),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidMimeType)
	})

	t.Run("should return ErrorFileTooLarge when file exceeds limit", func(t *testing.T) {
		svc := newService(
			&mocks.MockUserDocumentRepositoryRead{},
			&mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{},
			&mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{},
			&mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{},
		)

		_, err := svc.UploadUserDocument(context.Background(), serviceInterfaces.UploadUserDocumentInput{
			UserID:        "user-1",
			DocumentType:  "idCardFront",
			MimeType:      "image/jpeg",
			FileSizeBytes: 11 * 1024 * 1024, // 11 MB
			Data:          bytes.NewReader([]byte("fake")),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorFileTooLarge)
	})

	t.Run("should return ErrorUploadFailed when storage fails", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, "image/jpeg", int64(100)).
			Return("", fileErrors.ErrorUploadFailed)

		svc := newService(userDocRead, userDocWrite, vehicleDocRead, vehicleDocWrite, reviewRead, reviewWrite, storage)

		_, err := svc.UploadUserDocument(context.Background(), serviceInterfaces.UploadUserDocumentInput{
			UserID:        "user-1",
			DocumentType:  "idCardFront",
			MimeType:      "image/jpeg",
			FileSizeBytes: 100,
			Data:          bytes.NewReader([]byte("fake")),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorUploadFailed)
	})
}

// =============================================================================
// GetUserDocuments / GetUserDocument / GetCurrentUserDocument
// =============================================================================

func TestFileService_GetUserDocuments(t *testing.T) {
	t.Run("should return documents for user", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		docs := []*domain.UserDocument{
			stubUserDoc("doc-1", "user-1", "idCardFront"),
			stubUserDoc("doc-2", "user-1", "passport"),
		}
		userDocRead.On("GetByUserID", mock.Anything, "user-1").Return(docs, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetUserDocuments(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("should return empty slice when no documents", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocRead.On("GetByUserID", mock.Anything, "user-1").Return([]*domain.UserDocument{}, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetUserDocuments(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestFileService_GetUserDocument(t *testing.T) {
	t.Run("should return document by ID", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetUserDocument(context.Background(), "doc-1")

		require.NoError(t, err)
		assert.Equal(t, "doc-1", result.DocumentID)
	})

	t.Run("should return error when document not found", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocRead.On("GetByID", mock.Anything, "missing").Return(nil, fileErrors.ErrorDocumentNotFound)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetUserDocument(context.Background(), "missing")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

func TestFileService_GetCurrentUserDocument(t *testing.T) {
	t.Run("should return current document", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "idCardFront").Return(doc, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetCurrentUserDocument(context.Background(), "user-1", "idCardFront")

		require.NoError(t, err)
		assert.Equal(t, "doc-1", result.DocumentID)
	})

	t.Run("should return ErrorInvalidDocumentType for invalid type", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.GetCurrentUserDocument(context.Background(), "user-1", "badType")

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})
}

// =============================================================================
// GetDocument
// =============================================================================

func TestFileService_GetDocument(t *testing.T) {
	t.Run("should return document for owner", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetDocument(context.Background(), serviceInterfaces.GetDocumentInput{
			FileID: "doc-1",
			UserID: "user-1",
		})

		require.NoError(t, err)
		assert.Equal(t, "doc-1", result.FileID)
		assert.Equal(t, "idCardFront", result.FileType)
	})

	t.Run("should return document for support without ownership check", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetDocument(context.Background(), serviceInterfaces.GetDocumentInput{
			FileID:    "doc-1",
			SupportID: "support-1",
		})

		require.NoError(t, err)
		assert.Equal(t, "doc-1", result.FileID)
	})

	t.Run("should return ErrorUnauthorized for non-owner", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		doc := stubUserDoc("doc-1", "owner-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.GetDocument(context.Background(), serviceInterfaces.GetDocumentInput{
			FileID: "doc-1",
			UserID: "other-user",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorUnauthorized)
	})

	t.Run("should return ErrorDocumentNotFound for empty FileID", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.GetDocument(context.Background(), serviceInterfaces.GetDocumentInput{
			FileID: "",
			UserID: "user-1",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

// =============================================================================
// DeleteFile / DeleteUserDocument
// =============================================================================

func TestFileService_DeleteFile(t *testing.T) {
	t.Run("should delete file for owner", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		storage.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil)
		userDocWrite.On("Delete", mock.Anything, "doc-1").Return(nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.DeleteFile(context.Background(), serviceInterfaces.DeleteFileInput{
			UserID: "user-1",
			FileID: "doc-1",
		})

		require.NoError(t, err)
		userDocWrite.AssertExpectations(t)
	})

	t.Run("should return ErrorDocumentNotFound when document does not exist", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocRead.On("GetByID", mock.Anything, "missing").Return(nil, fileErrors.ErrorDocumentNotFound)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.DeleteFile(context.Background(), serviceInterfaces.DeleteFileInput{
			UserID: "user-1",
			FileID: "missing",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})

	t.Run("should return ErrorUnauthorized for non-owner", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		doc := stubUserDoc("doc-1", "owner-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.DeleteFile(context.Background(), serviceInterfaces.DeleteFileInput{
			UserID: "other-user",
			FileID: "doc-1",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorUnauthorized)
	})
}

func TestFileService_DeleteUserDocument(t *testing.T) {
	t.Run("should delete document and S3 object", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		storage.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil)
		userDocWrite.On("Delete", mock.Anything, "doc-1").Return(nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.DeleteUserDocument(context.Background(), "doc-1")

		require.NoError(t, err)
		storage.AssertExpectations(t)
		userDocWrite.AssertExpectations(t)
	})

	t.Run("should continue db delete even if S3 delete fails", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		storage.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(fileErrors.ErrorInternalServer)
		userDocWrite.On("Delete", mock.Anything, "doc-1").Return(nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.DeleteUserDocument(context.Background(), "doc-1")

		require.NoError(t, err)
		userDocWrite.AssertExpectations(t)
	})

	t.Run("should return error when document not found", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocRead.On("GetByID", mock.Anything, "missing").Return(nil, fileErrors.ErrorDocumentNotFound)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.DeleteUserDocument(context.Background(), "missing")

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

// =============================================================================
// UploadVehicleDocument
// =============================================================================

func TestFileService_UploadVehicleDocument(t *testing.T) {
	t.Run("should upload and return vehicle document", func(t *testing.T) {
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, "image/jpeg", int64(200)).
			Return("https://storage.example.com/insurance/vehicle-1/doc.jpg", nil)
		vehicleDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.VehicleDocument")).
			Return("vdoc-1", nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			vehicleDocRead, vehicleDocWrite,
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		result, err := svc.UploadVehicleDocument(context.Background(), serviceInterfaces.UploadVehicleDocumentInput{
			VehicleID:     "vehicle-1",
			DocumentName:  "insurance",
			DocumentType:  "insurance",
			MimeType:      "image/jpeg",
			FileSizeBytes: 200,
			Data:          bytes.NewReader([]byte("fake")),
		})

		require.NoError(t, err)
		assert.NotEmpty(t, result.DocumentID)
		assert.Equal(t, "vehicle-1", result.VehicleID)
		assert.Equal(t, "insurance", result.DocumentType)
		assert.Equal(t, "pending", result.Status)
	})

	t.Run("should return ErrorInvalidDocumentType for invalid vehicle type", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.UploadVehicleDocument(context.Background(), serviceInterfaces.UploadVehicleDocumentInput{
			VehicleID:     "vehicle-1",
			DocumentType:  "idCardFront", // type utilisateur, invalide pour véhicule
			MimeType:      "image/jpeg",
			FileSizeBytes: 100,
			Data:          bytes.NewReader([]byte("fake")),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})

	t.Run("should return ErrorFileTooLarge", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.UploadVehicleDocument(context.Background(), serviceInterfaces.UploadVehicleDocumentInput{
			VehicleID:     "vehicle-1",
			DocumentType:  "insurance",
			MimeType:      "image/jpeg",
			FileSizeBytes: 11 * 1024 * 1024,
			Data:          bytes.NewReader([]byte("fake")),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorFileTooLarge)
	})
}

// =============================================================================
// GetVehicleDocuments / GetVehicleDocument
// =============================================================================

func TestFileService_GetVehicleDocuments(t *testing.T) {
	t.Run("should return vehicle documents", func(t *testing.T) {
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		docs := []*domain.VehicleDocument{
			stubVehicleDoc("vdoc-1", "vehicle-1", "insurance"),
		}
		vehicleDocRead.On("GetByVehicleID", mock.Anything, "vehicle-1").Return(docs, nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			vehicleDocRead, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetVehicleDocuments(context.Background(), "vehicle-1")

		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
}

func TestFileService_GetVehicleDocument(t *testing.T) {
	t.Run("should return vehicle document by ID", func(t *testing.T) {
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		doc := stubVehicleDoc("vdoc-1", "vehicle-1", "insurance")
		vehicleDocRead.On("GetByID", mock.Anything, "vdoc-1").Return(doc, nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			vehicleDocRead, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetVehicleDocument(context.Background(), "vdoc-1")

		require.NoError(t, err)
		assert.Equal(t, "vdoc-1", result.DocumentID)
	})
}

// =============================================================================
// DeleteVehicleDocument
// =============================================================================

func TestFileService_DeleteVehicleDocument(t *testing.T) {
	t.Run("should delete vehicle document", func(t *testing.T) {
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		doc := stubVehicleDoc("vdoc-1", "vehicle-1", "insurance")
		vehicleDocRead.On("GetByID", mock.Anything, "vdoc-1").Return(doc, nil)
		storage.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil)
		vehicleDocWrite.On("Delete", mock.Anything, "vdoc-1").Return(nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			vehicleDocRead, vehicleDocWrite,
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.DeleteVehicleDocument(context.Background(), "vdoc-1")

		require.NoError(t, err)
		vehicleDocWrite.AssertExpectations(t)
	})

	t.Run("should return error when document not found", func(t *testing.T) {
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		vehicleDocRead.On("GetByID", mock.Anything, "missing").Return(nil, fileErrors.ErrorDocumentNotFound)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			vehicleDocRead, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.DeleteVehicleDocument(context.Background(), "missing")

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

// =============================================================================
// ChangeDocument
// =============================================================================

func TestFileService_ChangeDocument(t *testing.T) {
	t.Run("should replace document successfully", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		existing := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(existing, nil)
		// UploadUserDocument appellera GetCurrentByUserIDAndType et Create
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "idCardFront").
			Return(existing, nil)
		userDocWrite.On("MarkAsReplaced", mock.Anything, "doc-1", mock.AnythingOfType("string")).Return(nil)
		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/idCardFront/user-1/new.jpg", nil)
		userDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).Return("new-doc", nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.ChangeDocument(context.Background(), serviceInterfaces.ChangeDocumentInput{
			UserID:      "user-1",
			FileID:      "doc-1",
			NewDocument: fakeJPEG(),
		})

		require.NoError(t, err)
	})

	t.Run("should return ErrorInvalidDocumentType for empty new document", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.ChangeDocument(context.Background(), serviceInterfaces.ChangeDocumentInput{
			UserID:      "user-1",
			FileID:      "doc-1",
			NewDocument: nil,
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})

	t.Run("should return ErrorDocumentNotFound when document does not exist", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocRead.On("GetByID", mock.Anything, "missing").Return(nil, fileErrors.ErrorDocumentNotFound)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.ChangeDocument(context.Background(), serviceInterfaces.ChangeDocumentInput{
			UserID:      "user-1",
			FileID:      "missing",
			NewDocument: fakeJPEG(),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})

	t.Run("should return ErrorDocumentNotFound when user does not own document", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		doc := stubUserDoc("doc-1", "owner-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.ChangeDocument(context.Background(), serviceInterfaces.ChangeDocumentInput{
			UserID:      "other-user",
			FileID:      "doc-1",
			NewDocument: fakeJPEG(),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

// =============================================================================
// UploadIdDocument
// =============================================================================

func TestFileService_UploadIdDocument(t *testing.T) {
	t.Run("should upload IDCard (recto + verso)", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/idCardFront/user-1/doc.jpg", nil)
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
			Return(nil, fileErrors.ErrorDocumentNotFound)
		userDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Return("doc-id", nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.UploadIdDocument(context.Background(), serviceInterfaces.UploadIdDocumentInput{
			UserID:       "user-1",
			DocumentType: "IDCard",
			IDCardRecto:  fakeJPEG(),
			IDCardVerso:  fakeJPEG(),
		})

		require.NoError(t, err)
		// 2 uploads : recto + verso
		storage.AssertNumberOfCalls(t, "Upload", 2)
	})

	t.Run("should upload Passport", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/passport/user-1/doc.jpg", nil)
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
			Return(nil, fileErrors.ErrorDocumentNotFound)
		userDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Return("doc-id", nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.UploadIdDocument(context.Background(), serviceInterfaces.UploadIdDocumentInput{
			UserID:       "user-1",
			DocumentType: "Passport",
			Passport:     fakeJPEG(),
		})

		require.NoError(t, err)
		storage.AssertNumberOfCalls(t, "Upload", 1)
	})

	t.Run("should return ErrorInvalidDocumentType for empty UserID", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.UploadIdDocument(context.Background(), serviceInterfaces.UploadIdDocumentInput{
			UserID:       "",
			DocumentType: "IDCard",
			IDCardRecto:  fakeJPEG(),
			IDCardVerso:  fakeJPEG(),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})

	t.Run("should return ErrorInvalidDocumentType for IDCard missing verso", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.UploadIdDocument(context.Background(), serviceInterfaces.UploadIdDocumentInput{
			UserID:       "user-1",
			DocumentType: "IDCard",
			IDCardRecto:  fakeJPEG(),
			IDCardVerso:  nil, // manquant
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})

	t.Run("should return ErrorInvalidDocumentType for unknown DocumentType", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.UploadIdDocument(context.Background(), serviceInterfaces.UploadIdDocumentInput{
			UserID:       "user-1",
			DocumentType: "UnknownType",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})
}

// =============================================================================
// UploadVehicleDocuments
// =============================================================================

func TestFileService_UploadVehicleDocuments(t *testing.T) {
	t.Run("should upload 3 vehicle documents", func(t *testing.T) {
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/insurance/vehicle-1/doc.jpg", nil)
		vehicleDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.VehicleDocument")).
			Return("vdoc-id", nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			vehicleDocRead, vehicleDocWrite,
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.UploadVehicleDocuments(context.Background(), serviceInterfaces.UploadVehicleDocumentsInput{
			UserID:              "user-1",
			VehicleID:           "vehicle-1",
			DriverLicenceImage:  fakeJPEG(),
			Assurance:           fakeJPEG(),
			VehicleRegistration: fakeJPEG(),
		})

		require.NoError(t, err)
		storage.AssertNumberOfCalls(t, "Upload", 3)
	})

	t.Run("should return ErrorInvalidDocumentType for empty VehicleID", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.UploadVehicleDocuments(context.Background(), serviceInterfaces.UploadVehicleDocumentsInput{
			UserID:    "user-1",
			VehicleID: "",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})

	t.Run("should return ErrorInvalidDocumentType when a file is missing", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		err := svc.UploadVehicleDocuments(context.Background(), serviceInterfaces.UploadVehicleDocumentsInput{
			UserID:              "user-1",
			VehicleID:           "vehicle-1",
			DriverLicenceImage:  nil, // premier dans l'ordre — déclenche l'erreur immédiatement
			Assurance:           fakeJPEG(),
			VehicleRegistration: fakeJPEG(),
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidDocumentType)
	})

	t.Run("driver licence est stocke avec docType=driverLicence (non insurance)", func(t *testing.T) {
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}
		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/file.jpg", nil)

		captured := make([]*domain.VehicleDocument, 0, 3)
		vehicleDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.VehicleDocument")).
			Run(func(args mock.Arguments) {
				doc := args.Get(1).(*domain.VehicleDocument)
				captured = append(captured, doc)
			}).
			Return("vdoc-id", nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, vehicleDocWrite,
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.UploadVehicleDocuments(context.Background(), serviceInterfaces.UploadVehicleDocumentsInput{
			UserID:              "user-1",
			VehicleID:           "vehicle-1",
			FirstName:           "Jean",
			LastName:            "Dupont",
			DriverLicenceImage:  fakeJPEG(),
			Assurance:           fakeJPEG(),
			VehicleRegistration: fakeJPEG(),
		})

		require.NoError(t, err)
		require.Len(t, captured, 3)
		types := []string{captured[0].DocumentType, captured[1].DocumentType, captured[2].DocumentType}
		assert.Contains(t, types, "driverLicence")
		assert.Contains(t, types, "insurance")
		assert.Contains(t, types, "registrationCard")
		// le permis ne doit plus etre stocke comme insurance (bug du copier-coller)
		insuranceCount := 0
		for _, t := range types {
			if t == "insurance" {
				insuranceCount++
			}
		}
		assert.Equal(t, 1, insuranceCount, "un seul doc doit etre de type insurance")
	})

	t.Run("docName suit le format nom_prenom_date_heure_type", func(t *testing.T) {
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}
		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/file.jpg", nil)

		captured := make([]*domain.VehicleDocument, 0, 3)
		vehicleDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.VehicleDocument")).
			Run(func(args mock.Arguments) {
				captured = append(captured, args.Get(1).(*domain.VehicleDocument))
			}).
			Return("vdoc-id", nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, vehicleDocWrite,
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.UploadVehicleDocuments(context.Background(), serviceInterfaces.UploadVehicleDocumentsInput{
			UserID:              "user-1",
			VehicleID:           "vehicle-1",
			FirstName:           "Jean",
			LastName:            "Dupont",
			DriverLicenceImage:  fakeJPEG(),
			Assurance:           fakeJPEG(),
			VehicleRegistration: fakeJPEG(),
		})

		require.NoError(t, err)
		require.Len(t, captured, 3)

		re := regexp.MustCompile(`^dupont_jean_\d{8}_\d{6}_(driver_licence|assurance|vehicle_registration)$`)
		for _, doc := range captured {
			assert.Regexp(t, re, doc.DocumentName, "docName doit matcher le format")
		}
	})

	t.Run("sanitize les noms accentues et apostrophes dans le docName", func(t *testing.T) {
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		storage := &mocks.MockStorageClient{}
		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/file.jpg", nil)

		captured := make([]*domain.VehicleDocument, 0, 3)
		vehicleDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.VehicleDocument")).
			Run(func(args mock.Arguments) {
				captured = append(captured, args.Get(1).(*domain.VehicleDocument))
			}).
			Return("vdoc-id", nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, vehicleDocWrite,
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			storage)

		err := svc.UploadVehicleDocuments(context.Background(), serviceInterfaces.UploadVehicleDocumentsInput{
			UserID:              "user-1",
			VehicleID:           "vehicle-1",
			FirstName:           "Anne-Marie",
			LastName:            "Dupré",
			DriverLicenceImage:  fakeJPEG(),
			Assurance:           fakeJPEG(),
			VehicleRegistration: fakeJPEG(),
		})

		require.NoError(t, err)
		require.Len(t, captured, 3)
		re := regexp.MustCompile(`^dupre_anne-marie_\d{8}_\d{6}_(driver_licence|assurance|vehicle_registration)$`)
		for _, doc := range captured {
			assert.Regexp(t, re, doc.DocumentName)
		}
	})
}

// =============================================================================
// CreateDocumentReview
// =============================================================================

func TestFileService_CreateDocumentReview(t *testing.T) {
	t.Run("should create review for user document and update status to approved", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}

		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		reviewWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).Return("review-1", nil)
		// Post-creation : re-fetch + update status
		docCopy := *doc
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(&docCopy, nil)
		userDocWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Return(&docCopy, nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, reviewWrite,
			&mocks.MockStorageClient{})

		result, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID: "doc-1",
			Decision:       "approved",
			ReviewType:     "manual",
			ReviewedBy:     "admin-1",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, result.ReviewID)
		assert.Equal(t, "approved", result.Decision)
		assert.Equal(t, "pending", result.Status) // défaut
	})

	t.Run("should return ErrorMissingDocumentReference when no document ID", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			Decision:   "approved",
			ReviewType: "manual",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorMissingDocumentReference)
	})

	t.Run("should return ErrorMultipleDocumentReference when both IDs provided", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID:    "doc-1",
			VehicleDocumentID: "vdoc-1",
			Decision:          "approved",
			ReviewType:        "manual",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorMultipleDocumentReference)
	})

	t.Run("should return ErrorInvalidReviewDecision for invalid decision", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID: "doc-1",
			Decision:       "invalid_decision",
			ReviewType:     "manual",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidReviewDecision)
	})

	t.Run("should return ErrorInvalidReviewType for invalid review type", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID: "doc-1",
			Decision:       "approved",
			ReviewType:     "robot", // invalide
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidReviewType)
	})

	t.Run("should return ErrorInvalidReviewStatus for invalid status", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID: "doc-1",
			Decision:       "approved",
			ReviewType:     "manual",
			Status:         "bad_status",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidReviewStatus)
	})

	t.Run("should return ErrorInvalidReasonRejection for invalid reason", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID:  "doc-1",
			Decision:        "approved",
			ReviewType:      "manual",
			ReasonRejection: "bad_reason",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInvalidReasonRejection)
	})

	t.Run("should return ErrorDocumentNotFound when user document does not exist", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocRead.On("GetByID", mock.Anything, "missing").Return(nil, fileErrors.ErrorDocumentNotFound)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID: "missing",
			Decision:       "approved",
			ReviewType:     "manual",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})

	t.Run("should return ErrorInternalServer on invalid SessionExpiresAt format", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)

		svc := newService(userDocRead, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID:   "doc-1",
			Decision:         "approved",
			ReviewType:       "manual",
			SessionExpiresAt: "not-a-date",
		})

		assert.ErrorIs(t, err, fileErrors.ErrorInternalServer)
	})

	t.Run("should map rejection decision to rejected status on document", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}

		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		reviewWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).Return("review-1", nil)

		var updatedDoc *domain.UserDocument
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		userDocWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Run(func(args mock.Arguments) {
				updatedDoc = args.Get(1).(*domain.UserDocument)
			}).
			Return(doc, nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, reviewWrite,
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID:  "doc-1",
			Decision:        "rejected",
			ReviewType:      "manual",
			ReasonRejection: "document_expired",
		})

		require.NoError(t, err)
		require.NotNil(t, updatedDoc)
		assert.Equal(t, "rejected", updatedDoc.Status)
	})

	t.Run("should default AttemptNumber to 1 when 0", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}

		doc := stubUserDoc("doc-1", "user-1", "idCardFront")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)

		var capturedReview *domain.DocumentReview
		reviewWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).
			Run(func(args mock.Arguments) {
				capturedReview = args.Get(1).(*domain.DocumentReview)
			}).
			Return("review-1", nil)
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		userDocWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).Return(doc, nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, reviewWrite,
			&mocks.MockStorageClient{})

		_, err := svc.CreateDocumentReview(context.Background(), serviceInterfaces.CreateReviewInput{
			UserDocumentID: "doc-1",
			Decision:       "approved",
			ReviewType:     "manual",
			AttemptNumber:  0, // doit devenir 1
		})

		require.NoError(t, err)
		assert.Equal(t, int16(1), capturedReview.AttemptNumber)
	})
}

// =============================================================================
// GetDocumentReviews
// =============================================================================

func TestFileService_GetDocumentReviews(t *testing.T) {
	t.Run("should return reviews by user document ID", func(t *testing.T) {
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		docID := "doc-1"
		reviews := []*domain.DocumentReview{
			{ReviewID: "review-1", UserDocumentID: &docID, Decision: "approved"},
		}
		reviewRead.On("GetByUserDocumentID", mock.Anything, "doc-1").Return(reviews, nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			reviewRead, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetDocumentReviews(context.Background(), "doc-1", "")

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "review-1", result[0].ReviewID)
	})

	t.Run("should return reviews by vehicle document ID", func(t *testing.T) {
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		vDocID := "vdoc-1"
		reviews := []*domain.DocumentReview{
			{ReviewID: "review-2", VehicleDocumentID: &vDocID, Decision: "rejected"},
		}
		reviewRead.On("GetByVehicleDocumentID", mock.Anything, "vdoc-1").Return(reviews, nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			reviewRead, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		result, err := svc.GetDocumentReviews(context.Background(), "", "vdoc-1")

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "review-2", result[0].ReviewID)
	})

	t.Run("should return ErrorDocumentNotFound when both IDs are empty", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.GetDocumentReviews(context.Background(), "", "")

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

// =============================================================================
// Domain helpers validation (extensionFromMimeType via upload)
// =============================================================================

func TestFileService_MimeTypeExtensions(t *testing.T) {
	mimeTypes := []struct {
		mime string
		ext  string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"application/pdf", ".pdf"},
	}

	for _, tc := range mimeTypes {
		t.Run("extension for "+tc.mime, func(t *testing.T) {
			userDocRead := &mocks.MockUserDocumentRepositoryRead{}
			userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
			storage := &mocks.MockStorageClient{}

			var capturedKey string
			storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, tc.mime, int64(100)).
				Run(func(args mock.Arguments) {
					capturedKey = args.String(1)
				}).
				Return("https://storage.example.com/key", nil)
			userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
				Return(nil, fileErrors.ErrorDocumentNotFound)
			userDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
				Return("doc-id", nil)

			svc := newService(userDocRead, userDocWrite,
				&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
				&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
				storage)

			_, err := svc.UploadUserDocument(context.Background(), serviceInterfaces.UploadUserDocumentInput{
				UserID:        "user-1",
				DocumentType:  "idCardFront",
				MimeType:      tc.mime,
				FileSizeBytes: 100,
				Data:          bytes.NewReader([]byte("fake")),
			})

			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(capturedKey, tc.ext),
				"expected key %q to end with %q", capturedKey, tc.ext)
		})
	}
}
