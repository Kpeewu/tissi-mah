package handler_test

import (
	"context"
	"testing"

	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestGetUserByFirebaseID_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		mockSvc, handler := newMockAndHandler()
		u := newDomainUser()
		mockSvc.On("GetUserByFirebaseID", mock.Anything, u.FirebaseID).Return(u, nil)

		resp, err := handler.GetUserByFirebaseID(context.Background(), &userpb.GetUserByFirebaseIDRequest{
			FirebaseID: u.FirebaseID,
		})
		require.NoError(t, err)
		assert.Equal(t, u.UserID, resp.UserID)
	})

	t.Run("not found", func(t *testing.T) {
		mockSvc, handler := newMockAndHandler()
		mockSvc.On("GetUserByFirebaseID", mock.Anything, "bad").Return(nil, userErrors.ErrorUserNotFound)
		_, err := handler.GetUserByFirebaseID(context.Background(), &userpb.GetUserByFirebaseIDRequest{FirebaseID: "bad"})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

func TestGetUserByUserID_Handler(t *testing.T) {
	t.Run("succès - enrichi avec email/phone", func(t *testing.T) {
		mockSvc, handler := newMockAndHandler()
		u := newDomainUser()
		mockSvc.On("GetUserProfileByUserID", mock.Anything, u.UserID).
			Return(u, "john@example.com", "+22890000000", nil)

		resp, err := handler.GetUserByUserID(context.Background(), &userpb.GetUserByUserIDRequest{UserID: u.UserID})
		require.NoError(t, err)
		assert.Equal(t, u.UserID, resp.UserID)
		assert.Equal(t, "john@example.com", resp.Email)
		assert.Equal(t, "+22890000000", resp.PhoneNumber)
	})

	t.Run("not found", func(t *testing.T) {
		mockSvc, handler := newMockAndHandler()
		mockSvc.On("GetUserProfileByUserID", mock.Anything, "bad").
			Return(nil, "", "", userErrors.ErrorUserNotFound)
		_, err := handler.GetUserByUserID(context.Background(), &userpb.GetUserByUserIDRequest{UserID: "bad"})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

func TestSoftDeleteUser_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		mockSvc, handler := newMockAndHandler()
		mockSvc.On("SoftDeleteUser", mock.Anything, "auth-1").Return(nil)
		resp, err := handler.SoftDeleteUser(context.Background(), &userpb.SoftDeleteUserRequest{AuthID: "auth-1"})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("authID invalide → InvalidArgument", func(t *testing.T) {
		mockSvc, handler := newMockAndHandler()
		mockSvc.On("SoftDeleteUser", mock.Anything, "").Return(userErrors.ErrorInvalidUserID)
		_, err := handler.SoftDeleteUser(context.Background(), &userpb.SoftDeleteUserRequest{AuthID: ""})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("not found", func(t *testing.T) {
		mockSvc, handler := newMockAndHandler()
		mockSvc.On("SoftDeleteUser", mock.Anything, "auth-x").Return(userErrors.ErrorUserNotFound)
		_, err := handler.SoftDeleteUser(context.Background(), &userpb.SoftDeleteUserRequest{AuthID: "auth-x"})
		assertGRPCCode(t, err, codes.NotFound)
	})
}
