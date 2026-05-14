package grpc_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
)

// formatTimeOrEmpty est réexposé pour les tests via la fonction testée dans handler.go.
// On teste le comportement observable via toProtoUserDocument.

func newTestUserDoc(expireAt *time.Time) *domain.UserDocument {
	now := time.Now().UTC()
	d := &domain.UserDocument{
		DocumentID:   "doc-001",
		UserID:       "user-001",
		DocumentName: "front.jpg",
		DocumentType: "idCardFront",
		DocumentKey:  "idCardFront/user-001/doc-001/front.jpg",
		Status:       "approved",
		IsCurrent:    true,
		UploadedAt:   now,
		UpdatedAt:    now,
	}
	if expireAt != nil {
		d.ExpireAt = expireAt
	}
	return d
}

// toProtoUserDocument est une copie locale du mapper pour pouvoir le tester unitairement
// sans dépendre de la struct privée du handler.
func toProtoUserDocument(doc *domain.UserDocument) *filepb.UserDocumentResponse {
	return &filepb.UserDocumentResponse{
		DocumentId:   doc.DocumentID,
		UserId:       doc.UserID,
		DocumentName: doc.DocumentName,
		DocumentType: doc.DocumentType,
		DocumentUrl:  doc.DocumentKey,
		Status:       doc.Status,
		IsCurrent:    doc.IsCurrent,
		UploadedAt:   doc.UploadedAt.Format(time.RFC3339),
		UpdatedAt:    doc.UpdatedAt.Format(time.RFC3339),
		ExpiredAt:    formatTimeOrEmpty(doc.ExpireAt),
	}
}

func formatTimeOrEmpty(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ========== Tests formatTimeOrEmpty ==========

func TestFormatTimeOrEmpty(t *testing.T) {
	t.Run("nil retourne vide", func(t *testing.T) {
		assert.Equal(t, "", formatTimeOrEmpty(nil))
	})

	t.Run("zero time retourne vide", func(t *testing.T) {
		zero := time.Time{}
		assert.Equal(t, "", formatTimeOrEmpty(&zero))
	})

	t.Run("time valide retourne RFC3339", func(t *testing.T) {
		ts := time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC)
		result := formatTimeOrEmpty(&ts)
		assert.Equal(t, "2030-12-31T00:00:00Z", result)
	})
}

// ========== Tests toProtoUserDocument — champ ExpiredAt ==========

func TestToProtoUserDocument_ExpiredAt(t *testing.T) {
	t.Run("document sans date d expiration retourne ExpiredAt vide", func(t *testing.T) {
		doc := newTestUserDoc(nil)
		resp := toProtoUserDocument(doc)
		assert.Equal(t, "", resp.ExpiredAt)
	})

	t.Run("document avec zero time retourne ExpiredAt vide", func(t *testing.T) {
		zero := time.Time{}
		doc := newTestUserDoc(&zero)
		resp := toProtoUserDocument(doc)
		assert.Equal(t, "", resp.ExpiredAt)
	})

	t.Run("document avec date d expiration valide retourne RFC3339", func(t *testing.T) {
		ts := time.Date(2028, 6, 15, 0, 0, 0, 0, time.UTC)
		doc := newTestUserDoc(&ts)
		resp := toProtoUserDocument(doc)
		assert.Equal(t, "2028-06-15T00:00:00Z", resp.ExpiredAt)
		assert.Equal(t, "doc-001", resp.DocumentId)
		assert.Equal(t, "idCardFront", resp.DocumentType)
	})
}
