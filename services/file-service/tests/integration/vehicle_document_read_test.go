package integration

import (
	"context"
	"testing"

	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVehicleDocumentRead_GetByID(t *testing.T) {
	t.Run("should return vehicle document when found", func(t *testing.T) {
		cleanTables(t)
		doc := newVehicleDoc("vehicle-1", "insurance")
		insertVehicleDoc(t, doc)

		result, err := newVehicleDocReadRepo().GetByID(context.Background(), doc.DocumentID)

		require.NoError(t, err)
		assert.Equal(t, doc.DocumentID, result.DocumentID)
		assert.Equal(t, doc.VehicleID, result.VehicleID)
		assert.Equal(t, "insurance", result.DocumentType)
		assert.True(t, result.IsCurrent)
	})

	t.Run("should return ErrorDocumentNotFound when not found", func(t *testing.T) {
		cleanTables(t)

		result, err := newVehicleDocReadRepo().GetByID(context.Background(), "nonexistent-id")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, fileErrors.ErrorDocumentNotFound)
	})
}

func TestVehicleDocumentRead_GetByVehicleID(t *testing.T) {
	t.Run("should return all documents for vehicle", func(t *testing.T) {
		cleanTables(t)
		doc1 := newVehicleDoc("vehicle-2", "insurance")
		doc2 := newVehicleDoc("vehicle-2", "registrationCard")
		insertVehicleDoc(t, doc1)
		insertVehicleDoc(t, doc2)

		results, err := newVehicleDocReadRepo().GetByVehicleID(context.Background(), "vehicle-2")

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("should return empty slice when no documents", func(t *testing.T) {
		cleanTables(t)

		results, err := newVehicleDocReadRepo().GetByVehicleID(context.Background(), "vehicle-no-docs")

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("should not return documents of other vehicles", func(t *testing.T) {
		cleanTables(t)
		insertVehicleDoc(t, newVehicleDoc("vehicle-A", "insurance"))
		insertVehicleDoc(t, newVehicleDoc("vehicle-B", "insurance"))

		results, err := newVehicleDocReadRepo().GetByVehicleID(context.Background(), "vehicle-A")

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "vehicle-A", results[0].VehicleID)
	})
}
