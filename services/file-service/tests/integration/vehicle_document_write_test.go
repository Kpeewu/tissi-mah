package integration

import (
	"context"
	"testing"

	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVehicleDocumentWrite_Create(t *testing.T) {
	t.Run("should create document and return document_id", func(t *testing.T) {
		cleanTables(t)
		doc := newVehicleDoc("vehicle-10", "insurance")

		docID, err := newVehicleDocWriteRepo().Create(context.Background(), doc)

		require.NoError(t, err)
		assert.Equal(t, doc.DocumentID, docID)

		fetched, err := newVehicleDocReadRepo().GetByID(context.Background(), docID)
		require.NoError(t, err)
		assert.Equal(t, doc.VehicleID, fetched.VehicleID)
		assert.Equal(t, "pending", fetched.Status)
	})

	t.Run("should return error on duplicate document_id", func(t *testing.T) {
		cleanTables(t)
		doc := newVehicleDoc("vehicle-10", "insurance")
		insertVehicleDoc(t, doc)

		_, err := newVehicleDocWriteRepo().Create(context.Background(), doc)

		assert.Error(t, err)
	})
}

func TestVehicleDocumentWrite_Update(t *testing.T) {
	t.Run("should update document fields and return updated document", func(t *testing.T) {
		cleanTables(t)
		doc := newVehicleDoc("vehicle-11", "insurance")
		insertVehicleDoc(t, doc)

		doc.Status = "approved"
		doc.DocumentName = "updated_insurance"

		updated, err := newVehicleDocWriteRepo().Update(context.Background(), doc)

		require.NoError(t, err)
		assert.Equal(t, "approved", updated.Status)
		assert.Equal(t, "updated_insurance", updated.DocumentName)
	})

	t.Run("should return ErrorDocumentNotFound for unknown document", func(t *testing.T) {
		cleanTables(t)
		doc := newVehicleDoc("vehicle-11", "insurance")
		doc.Status = "approved"

		result, err := newVehicleDocWriteRepo().Update(context.Background(), doc)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

func TestVehicleDocumentWrite_Delete(t *testing.T) {
	t.Run("should delete vehicle document", func(t *testing.T) {
		cleanTables(t)
		doc := newVehicleDoc("vehicle-12", "registrationCard")
		insertVehicleDoc(t, doc)

		err := newVehicleDocWriteRepo().Delete(context.Background(), doc.DocumentID)

		require.NoError(t, err)

		result, readErr := newVehicleDocReadRepo().GetByID(context.Background(), doc.DocumentID)
		assert.Nil(t, result)
		assert.ErrorIs(t, readErr, fileErrors.ErrorDocumentNotFound)
	})

	t.Run("should return ErrorDocumentNotFound for unknown document", func(t *testing.T) {
		cleanTables(t)

		err := newVehicleDocWriteRepo().Delete(context.Background(), "nonexistent-id")

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

func TestVehicleDocumentWrite_MarkAsReplaced(t *testing.T) {
	t.Run("should set is_current=false and replaced_by", func(t *testing.T) {
		cleanTables(t)
		oldDoc := newVehicleDoc("vehicle-13", "insurance")
		insertVehicleDoc(t, oldDoc)

		newDoc := newVehicleDoc("vehicle-13", "insurance")
		insertVehicleDoc(t, newDoc)

		err := newVehicleDocWriteRepo().MarkAsReplaced(context.Background(), oldDoc.DocumentID, newDoc.DocumentID)

		require.NoError(t, err)

		fetched, err := newVehicleDocReadRepo().GetByID(context.Background(), oldDoc.DocumentID)
		require.NoError(t, err)
		assert.False(t, fetched.IsCurrent)
		require.NotNil(t, fetched.ReplacedBy)
		assert.Equal(t, newDoc.DocumentID, *fetched.ReplacedBy)
	})

	t.Run("should return ErrorDocumentNotFound for unknown document", func(t *testing.T) {
		cleanTables(t)

		err := newVehicleDocWriteRepo().MarkAsReplaced(context.Background(), "nonexistent-id", "replacement-id")

		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}
