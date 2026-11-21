package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/user-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/client"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetUserByFirebaseID(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		mockRead, _, _, _, svc := newService()
		u := fixtures.NewTestUser()
		mockRead.On("GetByFirebaseID", mock.Anything, "firebase-1").Return(u, nil)

		got, err := svc.GetUserByFirebaseID(context.Background(), "firebase-1")
		require.NoError(t, err)
		assert.Equal(t, u.UserID, got.UserID)
	})

	t.Run("firebaseID vide → ErrorUserNotFound", func(t *testing.T) {
		_, _, _, _, svc := newService()
		_, err := svc.GetUserByFirebaseID(context.Background(), "")
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})

	t.Run("repo échoue → propagation", func(t *testing.T) {
		mockRead, _, _, _, svc := newService()
		mockRead.On("GetByFirebaseID", mock.Anything, "firebase-x").
			Return(nil, userErrors.ErrorUserNotFound)
		_, err := svc.GetUserByFirebaseID(context.Background(), "firebase-x")
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})
}

func TestGetUserByUserID(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		mockRead, _, _, _, svc := newService()
		u := fixtures.NewTestUser()
		mockRead.On("GetByUserID", mock.Anything, u.UserID).Return(u, nil)

		got, err := svc.GetUserByUserID(context.Background(), u.UserID)
		require.NoError(t, err)
		assert.Equal(t, u.UserID, got.UserID)
	})

	t.Run("userID vide", func(t *testing.T) {
		_, _, _, _, svc := newService()
		_, err := svc.GetUserByUserID(context.Background(), "")
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})
}

func TestGetUserProfileByUserID(t *testing.T) {
	t.Run("succès - enrichi avec email/phone", func(t *testing.T) {
		mockRead, _, mockAuth, _, svc := newService()
		u := fixtures.NewTestUser()
		mockRead.On("GetByUserID", mock.Anything, u.UserID).Return(u, nil)
		mockAuth.On("GetAuthInfo", mock.Anything, u.AuthID).Return(&client.AuthInfo{
			Email: "a@b.c", PhoneNumber: "+100",
		}, nil)

		got, email, phone, err := svc.GetUserProfileByUserID(context.Background(), u.UserID)
		require.NoError(t, err)
		assert.Equal(t, u.UserID, got.UserID)
		assert.Equal(t, "a@b.c", email)
		assert.Equal(t, "+100", phone)
	})

	t.Run("dégradation gracieuse - auth échoue", func(t *testing.T) {
		mockRead, _, mockAuth, _, svc := newService()
		u := fixtures.NewTestUser()
		mockRead.On("GetByUserID", mock.Anything, u.UserID).Return(u, nil)
		mockAuth.On("GetAuthInfo", mock.Anything, u.AuthID).
			Return((*client.AuthInfo)(nil), errors.New("auth down"))

		got, email, phone, err := svc.GetUserProfileByUserID(context.Background(), u.UserID)
		require.NoError(t, err)
		assert.Equal(t, u.UserID, got.UserID)
		assert.Empty(t, email)
		assert.Empty(t, phone)
	})

	t.Run("user introuvable → propagation", func(t *testing.T) {
		mockRead, _, _, _, svc := newService()
		mockRead.On("GetByUserID", mock.Anything, "bad").Return(nil, userErrors.ErrorUserNotFound)
		_, _, _, err := svc.GetUserProfileByUserID(context.Background(), "bad")
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})
}

func TestSoftDeleteUser(t *testing.T) {
	t.Run("succès - anonymise puis delete", func(t *testing.T) {
		mockRead, mockWrite, _, _, svc := newService()
		u := fixtures.NewTestUser()
		mockRead.On("GetByAuthID", mock.Anything, u.AuthID).Return(u, nil)
		mockWrite.On("AnonymizeAndDelete", mock.Anything, u).Return(nil)

		err := svc.SoftDeleteUser(context.Background(), u.AuthID)
		require.NoError(t, err)
		assert.True(t, u.IsDeleted())
		mockWrite.AssertExpectations(t)
	})

	t.Run("authID vide → ErrorInvalidUserID", func(t *testing.T) {
		_, _, _, _, svc := newService()
		err := svc.SoftDeleteUser(context.Background(), "")
		assert.ErrorIs(t, err, userErrors.ErrorInvalidUserID)
	})

	t.Run("user introuvable → propagation", func(t *testing.T) {
		mockRead, _, _, _, svc := newService()
		mockRead.On("GetByAuthID", mock.Anything, "auth-x").
			Return(nil, userErrors.ErrorUserNotFound)
		err := svc.SoftDeleteUser(context.Background(), "auth-x")
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})

	t.Run("write échoue → propagation", func(t *testing.T) {
		mockRead, mockWrite, _, _, svc := newService()
		u := fixtures.NewTestUser()
		mockRead.On("GetByAuthID", mock.Anything, u.AuthID).Return(u, nil)
		mockWrite.On("AnonymizeAndDelete", mock.Anything, u).Return(userErrors.ErrorInternalServer)
		err := svc.SoftDeleteUser(context.Background(), u.AuthID)
		assert.ErrorIs(t, err, userErrors.ErrorInternalServer)
	})
}

func TestGetUserByAuthID_EmptyID(t *testing.T) {
	_, _, _, _, svc := newService()
	_, err := svc.GetUserByAuthID(context.Background(), "")
	assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
}
