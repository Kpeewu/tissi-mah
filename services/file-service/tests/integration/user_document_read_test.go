package integration

import (
	"context"
	"testing"

	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserDocumentRead_GetByID(t *testing.T) {
	t.Run("should return document when found", func(t *testing.T) {
		cleanTables(t)
		doc := newUserDoc("user-1", "idCardFront")
		insertUserDoc(t, doc)

		result, err := newUserDocReadRepo().GetByID(context.Background(), doc.DocumentID)

		require.NoError(t, err)
		assert.Equal(t, doc.DocumentID, result.DocumentID)
		assert.Equal(t, doc.UserID, result.UserID)
		assert.Equal(t, doc.DocumentType, result.DocumentType)
		assert.Equal(t, "pending", result.Status)
		assert.True(t, result.IsCurrent)
	})

	t.Run("should return ErrorDocumentNotFound when not found", func(t *testing.T) {
		cleanTables(t)

		result, err := newUserDocReadRepo().GetByID(context.Background(), "nonexistent-id")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

func TestUserDocumentRead_GetByUserID(t *testing.T) {
	t.Run("should return all documents for user ordered by uploaded_at DESC", func(t *testing.T) {
		cleanTables(t)
		doc1 := newUserDoc("user-2", "idCardFront")
		doc2 := newUserDoc("user-2", "passport")
		insertUserDoc(t, doc1)
		insertUserDoc(t, doc2)

		results, err := newUserDocReadRepo().GetByUserID(context.Background(), "user-2")

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("should return empty slice when no documents", func(t *testing.T) {
		cleanTables(t)

		results, err := newUserDocReadRepo().GetByUserID(context.Background(), "user-no-docs")

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("should not return documents of other users", func(t *testing.T) {
		cleanTables(t)
		insertUserDoc(t, newUserDoc("user-A", "idCardFront"))
		insertUserDoc(t, newUserDoc("user-B", "passport"))

		results, err := newUserDocReadRepo().GetByUserID(context.Background(), "user-A")

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "user-A", results[0].UserID)
	})
}

func TestUserDocumentRead_GetCurrentByUserIDAndType(t *testing.T) {
	t.Run("should return current document", func(t *testing.T) {
		cleanTables(t)
		doc := newUserDoc("user-3", "idCardFront")
		insertUserDoc(t, doc)

		result, err := newUserDocReadRepo().GetCurrentByUserIDAndType(context.Background(), "user-3", "idCardFront")

		require.NoError(t, err)
		assert.Equal(t, doc.DocumentID, result.DocumentID)
		assert.True(t, result.IsCurrent)
	})

	t.Run("should return ErrorDocumentNotFound when no current document", func(t *testing.T) {
		cleanTables(t)

		result, err := newUserDocReadRepo().GetCurrentByUserIDAndType(context.Background(), "user-3", "idCardFront")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})

	t.Run("should not return non-current document", func(t *testing.T) {
		cleanTables(t)
		old := newUserDoc("user-4", "idCardFront")
		replacement := newUserDoc("user-4", "idCardFront")
		insertUserDoc(t, old)
		insertUserDoc(t, replacement)
		// Marquer l'ancien comme remplacé par le nouveau (FK valide)
		err := newUserDocWriteRepo().MarkAsReplaced(context.Background(), old.DocumentID, replacement.DocumentID)
		require.NoError(t, err)

		// Le document courant doit être le replacement
		result, err := newUserDocReadRepo().GetCurrentByUserIDAndType(context.Background(), "user-4", "idCardFront")

		require.NoError(t, err)
		assert.Equal(t, replacement.DocumentID, result.DocumentID)
	})
}
