package integration

import (
	"context"
	"testing"

	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentReviewRead_GetByID(t *testing.T) {
	t.Run("should return review when found", func(t *testing.T) {
		cleanTables(t)
		userDoc := newUserDoc("user-rr-1", "idCardFront")
		insertUserDoc(t, userDoc)
		review := newReviewForUserDoc(userDoc.DocumentID)
		insertReview(t, review)

		result, err := newReviewReadRepo().GetByID(context.Background(), review.ReviewID)

		require.NoError(t, err)
		assert.Equal(t, review.ReviewID, result.ReviewID)
		assert.Equal(t, "approved", result.Decision)
		assert.Equal(t, "manual", result.ReviewType)
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, int16(1), result.AttemptNumber)
		require.NotNil(t, result.UserDocumentID)
		assert.Equal(t, userDoc.DocumentID, *result.UserDocumentID)
		assert.Nil(t, result.VehicleDocumentID)
	})

	t.Run("should return ErrorReviewNotFound when not found", func(t *testing.T) {
		cleanTables(t)

		result, err := newReviewReadRepo().GetByID(context.Background(), "nonexistent-review")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, fileErrors.ErrorReviewNotFound)
	})
}

func TestDocumentReviewRead_GetByUserDocumentID(t *testing.T) {
	t.Run("should return all reviews for user document", func(t *testing.T) {
		cleanTables(t)
		userDoc := newUserDoc("user-rr-2", "idCardFront")
		insertUserDoc(t, userDoc)
		review1 := newReviewForUserDoc(userDoc.DocumentID)
		review2 := newReviewForUserDoc(userDoc.DocumentID)
		insertReview(t, review1)
		insertReview(t, review2)

		results, err := newReviewReadRepo().GetByUserDocumentID(context.Background(), userDoc.DocumentID)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, r := range results {
			require.NotNil(t, r.UserDocumentID)
			assert.Equal(t, userDoc.DocumentID, *r.UserDocumentID)
		}
	})

	t.Run("should return empty slice when no reviews", func(t *testing.T) {
		cleanTables(t)
		userDoc := newUserDoc("user-rr-3", "passport")
		insertUserDoc(t, userDoc)

		results, err := newReviewReadRepo().GetByUserDocumentID(context.Background(), userDoc.DocumentID)

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("should not return reviews of other documents", func(t *testing.T) {
		cleanTables(t)
		docA := newUserDoc("user-rr-4", "idCardFront")
		docB := newUserDoc("user-rr-4", "passport")
		insertUserDoc(t, docA)
		insertUserDoc(t, docB)
		insertReview(t, newReviewForUserDoc(docA.DocumentID))
		insertReview(t, newReviewForUserDoc(docB.DocumentID))

		results, err := newReviewReadRepo().GetByUserDocumentID(context.Background(), docA.DocumentID)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, docA.DocumentID, *results[0].UserDocumentID)
	})
}

func TestDocumentReviewRead_GetByUserID(t *testing.T) {
	t.Run("should return reviews for both user and vehicle documents owned by user", func(t *testing.T) {
		cleanTables(t)
		userID := "user-rr-getall-1"

		// Document d'identité (user_documents)
		userDoc := newUserDoc(userID, "idCardFront")
		insertUserDoc(t, userDoc)
		userReview := newReviewForUserDoc(userDoc.DocumentID)
		insertReview(t, userReview)

		// Document véhicule (vehicle_documents) — owned by same user
		vehicleDoc := newVehicleDoc("vehicle-rr-getall-1", "insurance")
		vehicleDoc.UserID = userID
		insertVehicleDoc(t, vehicleDoc)
		vehicleReview := newReviewForVehicleDoc(vehicleDoc.DocumentID)
		insertReview(t, vehicleReview)

		results, err := newReviewReadRepo().GetByUserID(context.Background(), userID)

		require.NoError(t, err)
		assert.Len(t, results, 2, "GetByUserID doit retourner les reviews user ET véhicule")

		var foundUser, foundVehicle bool
		for _, r := range results {
			if r.UserDocumentID != nil && *r.UserDocumentID == userDoc.DocumentID {
				foundUser = true
			}
			if r.VehicleDocumentID != nil && *r.VehicleDocumentID == vehicleDoc.DocumentID {
				foundVehicle = true
			}
		}
		assert.True(t, foundUser, "review du document d'identité manquante")
		assert.True(t, foundVehicle, "review du document véhicule manquante")
	})

	t.Run("should not return reviews of vehicle documents owned by other users", func(t *testing.T) {
		cleanTables(t)
		callerID := "user-rr-getall-caller"
		otherID := "user-rr-getall-other"

		ownVehicle := newVehicleDoc("vehicle-own", "insurance")
		ownVehicle.UserID = callerID
		insertVehicleDoc(t, ownVehicle)
		insertReview(t, newReviewForVehicleDoc(ownVehicle.DocumentID))

		foreignVehicle := newVehicleDoc("vehicle-foreign", "insurance")
		foreignVehicle.UserID = otherID
		insertVehicleDoc(t, foreignVehicle)
		insertReview(t, newReviewForVehicleDoc(foreignVehicle.DocumentID))

		results, err := newReviewReadRepo().GetByUserID(context.Background(), callerID)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		require.NotNil(t, results[0].VehicleDocumentID)
		assert.Equal(t, ownVehicle.DocumentID, *results[0].VehicleDocumentID)
	})
}

func TestDocumentReviewRead_GetByVehicleDocumentID(t *testing.T) {
	t.Run("should return all reviews for vehicle document", func(t *testing.T) {
		cleanTables(t)
		vehicleDoc := newVehicleDoc("vehicle-rr-1", "insurance")
		insertVehicleDoc(t, vehicleDoc)
		review1 := newReviewForVehicleDoc(vehicleDoc.DocumentID)
		review2 := newReviewForVehicleDoc(vehicleDoc.DocumentID)
		insertReview(t, review1)
		insertReview(t, review2)

		results, err := newReviewReadRepo().GetByVehicleDocumentID(context.Background(), vehicleDoc.DocumentID)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, r := range results {
			require.NotNil(t, r.VehicleDocumentID)
			assert.Equal(t, vehicleDoc.DocumentID, *r.VehicleDocumentID)
		}
	})

	t.Run("should return empty slice when no reviews", func(t *testing.T) {
		cleanTables(t)
		vehicleDoc := newVehicleDoc("vehicle-rr-2", "registrationCard")
		insertVehicleDoc(t, vehicleDoc)

		results, err := newReviewReadRepo().GetByVehicleDocumentID(context.Background(), vehicleDoc.DocumentID)

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}
