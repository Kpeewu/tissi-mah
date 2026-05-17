package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentReviewWrite_Create(t *testing.T) {
	t.Run("should create review for user document and return review_id", func(t *testing.T) {
		cleanTables(t)
		userDoc := newUserDoc("user-review-1", "idCardFront")
		insertUserDoc(t, userDoc)

		review := newReviewForUserDoc(userDoc)

		reviewID, err := newReviewWriteRepo().Create(context.Background(), review)

		require.NoError(t, err)
		assert.Equal(t, review.ReviewID, reviewID)

		// Vérification en lecture
		fetched, err := newReviewReadRepo().GetByID(context.Background(), reviewID)
		require.NoError(t, err)
		assert.Equal(t, review.ReviewID, fetched.ReviewID)
		assert.Equal(t, "approved", fetched.Decision)
		assert.Equal(t, "manual", fetched.ReviewType)
		assert.Equal(t, int16(1), fetched.AttemptNumber)
		require.NotNil(t, fetched.UserDocumentID)
		assert.Equal(t, userDoc.DocumentID, *fetched.UserDocumentID)
	})

	t.Run("should create review for vehicle document", func(t *testing.T) {
		cleanTables(t)
		vehicleDoc := newVehicleDoc("vehicle-review-1", "insurance")
		insertVehicleDoc(t, vehicleDoc)

		review := newReviewForVehicleDoc(vehicleDoc)

		reviewID, err := newReviewWriteRepo().Create(context.Background(), review)

		require.NoError(t, err)
		assert.Equal(t, review.ReviewID, reviewID)

		fetched, err := newReviewReadRepo().GetByID(context.Background(), reviewID)
		require.NoError(t, err)
		assert.Equal(t, "rejected", fetched.Decision)
		assert.Equal(t, "document_expired", fetched.ReasonRejection)
		require.NotNil(t, fetched.VehicleDocumentID)
		assert.Equal(t, vehicleDoc.DocumentID, *fetched.VehicleDocumentID)
	})

	t.Run("should return error on duplicate review_id", func(t *testing.T) {
		cleanTables(t)
		userDoc := newUserDoc("user-review-2", "passport")
		insertUserDoc(t, userDoc)
		review := newReviewForUserDoc(userDoc)
		insertReview(t, review)

		_, err := newReviewWriteRepo().Create(context.Background(), review)

		assert.Error(t, err)
	})
}
