package integration

import (
	"context"
	"testing"

	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserDocumentWrite_Create(t *testing.T) {
	t.Run("should create document and return document_id", func(t *testing.T) {
		cleanTables(t)
		doc := newUserDoc("user-10", "idCardFront")

		docID, err := newUserDocWriteRepo().Create(context.Background(), doc)

		require.NoError(t, err)
		assert.Equal(t, doc.DocumentID, docID)

		// Vérification en lecture
		fetched, err := newUserDocReadRepo().GetByID(context.Background(), docID)
		require.NoError(t, err)
		assert.Equal(t, doc.UserID, fetched.UserID)
		assert.Equal(t, doc.DocumentType, fetched.DocumentType)
		assert.Equal(t, "pending", fetched.Status)
	})

	t.Run("should return error on duplicate document_id", func(t *testing.T) {
		cleanTables(t)
		doc := newUserDoc("user-10", "idCardFront")
		insertUserDoc(t, doc)

		_, err := newUserDocWriteRepo().Create(context.Background(), doc)

		assert.Error(t, err)
	})
}

func TestUserDocumentWrite_Update(t *testing.T) {
	t.Run("should update document fields and return updated document", func(t *testing.T) {
		cleanTables(t)
		doc := newUserDoc("user-11", "idCardFront")
		insertUserDoc(t, doc)

		doc.Status = "approved"
		doc.DocumentName = "updated_name"

		updated, err := newUserDocWriteRepo().Update(context.Background(), doc)

		require.NoError(t, err)
		assert.Equal(t, "approved", updated.Status)
		assert.Equal(t, "updated_name", updated.DocumentName)
		assert.Equal(t, doc.DocumentID, updated.DocumentID)
	})

	t.Run("should return ErrorDocumentNotFound for unknown document", func(t *testing.T) {
		cleanTables(t)
		doc := newUserDoc("user-11", "idCardFront")
		doc.Status = "approved"

		result, err := newUserDocWriteRepo().Update(context.Background(), doc)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

func TestUserDocumentWrite_Delete(t *testing.T) {
	t.Run("should delete document from database", func(t *testing.T) {
		cleanTables(t)
		doc := newUserDoc("user-12", "passport")
		insertUserDoc(t, doc)

		err := newUserDocWriteRepo().Delete(context.Background(), doc.DocumentID)

		require.NoError(t, err)

		// Vérification : le document n'existe plus
		result, readErr := newUserDocReadRepo().GetByID(context.Background(), doc.DocumentID)
		assert.Nil(t, result)
		assert.ErrorIs(t, readErr, fileErrors.ErrorDocumentNotFound)
	})

	t.Run("should return ErrorDocumentNotFound for unknown document", func(t *testing.T) {
		cleanTables(t)

		err := newUserDocWriteRepo().Delete(context.Background(), "nonexistent-id")

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

func TestUserDocumentWrite_MarkAsReplaced(t *testing.T) {
	t.Run("should set is_current=false and replaced_by", func(t *testing.T) {
		cleanTables(t)
		oldDoc := newUserDoc("user-13", "idCardFront")
		insertUserDoc(t, oldDoc)

		newDoc := newUserDoc("user-13", "idCardFront")
		insertUserDoc(t, newDoc)

		err := newUserDocWriteRepo().MarkAsReplaced(context.Background(), oldDoc.DocumentID, newDoc.DocumentID)

		require.NoError(t, err)

		fetched, err := newUserDocReadRepo().GetByID(context.Background(), oldDoc.DocumentID)
		require.NoError(t, err)
		assert.False(t, fetched.IsCurrent)
		require.NotNil(t, fetched.ReplacedBy)
		assert.Equal(t, newDoc.DocumentID, *fetched.ReplacedBy)
	})

	t.Run("should return ErrorDocumentNotFound for unknown document", func(t *testing.T) {
		cleanTables(t)

		err := newUserDocWriteRepo().MarkAsReplaced(context.Background(), "nonexistent-id", "replacement-id")

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}
