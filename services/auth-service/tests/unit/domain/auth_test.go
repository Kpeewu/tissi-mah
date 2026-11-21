package domain_test

import (
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/auth-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- GetEmail / GetPhoneNumber ---

func TestAuth_GetEmail(t *testing.T) {
	t.Run("retourne l'email quand il est défini", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithEmail("test@example.com"))
		assert.Equal(t, "test@example.com", auth.GetEmail())
	})

	t.Run("retourne une chaîne vide quand email est nil", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithNoEmail())
		assert.Equal(t, "", auth.GetEmail())
	})
}

func TestAuth_GetPhoneNumber(t *testing.T) {
	t.Run("retourne le numéro quand il est défini", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithPhoneNumber("+22890000000"))
		assert.Equal(t, "+22890000000", auth.GetPhoneNumber())
	})

	t.Run("retourne une chaîne vide quand phone est nil", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithNoPhoneNumber())
		assert.Equal(t, "", auth.GetPhoneNumber())
	})
}

// --- HasEmail / HasPhoneNumber ---

func TestAuth_HasEmail(t *testing.T) {
	t.Run("true quand email est défini", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithEmail("test@example.com"))
		assert.True(t, auth.HasEmail())
	})

	t.Run("false quand email est nil", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithNoEmail())
		assert.False(t, auth.HasEmail())
	})
}

func TestAuth_HasPhoneNumber(t *testing.T) {
	t.Run("true quand phone est défini", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithPhoneNumber("+22890000000"))
		assert.True(t, auth.HasPhoneNumber())
	})

	t.Run("false quand phone est nil", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithNoPhoneNumber())
		assert.False(t, auth.HasPhoneNumber())
	})
}

// --- IsDeleted ---

func TestAuth_IsDeleted(t *testing.T) {
	t.Run("false quand DeletedAt est nil", func(t *testing.T) {
		auth := fixtures.NewTestAuth()
		assert.False(t, auth.IsDeleted())
	})

	t.Run("true quand DeletedAt est défini", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithDeleted())
		assert.True(t, auth.IsDeleted())
	})
}

// --- IsSuspendedNow ---

func TestAuth_IsSuspendedNow(t *testing.T) {
	t.Run("false quand IsSuspended est false", func(t *testing.T) {
		auth := fixtures.NewTestAuth()
		assert.False(t, auth.IsSuspendedNow())
	})

	t.Run("true quand suspendu indéfiniment (pas de date de fin)", func(t *testing.T) {
		auth := fixtures.NewTestAuth()
		auth.IsSuspended = true
		auth.SuspensionEndDate = nil
		assert.True(t, auth.IsSuspendedNow())
	})

	t.Run("true quand la suspension est en cours", func(t *testing.T) {
		future := time.Now().UTC().Add(24 * time.Hour)
		auth := fixtures.NewTestAuth(fixtures.WithSuspended(future))
		assert.True(t, auth.IsSuspendedNow())
	})

	t.Run("false quand la suspension a expiré", func(t *testing.T) {
		past := time.Now().UTC().Add(-24 * time.Hour)
		auth := fixtures.NewTestAuth(fixtures.WithSuspended(past))
		assert.False(t, auth.IsSuspendedNow())
	})
}

// --- CanLogin ---

func TestAuth_CanLogin(t *testing.T) {
	t.Run("true quand actif, non suspendu, non supprimé", func(t *testing.T) {
		auth := fixtures.NewTestAuth()
		assert.True(t, auth.CanLogin())
	})

	t.Run("false quand supprimé", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithDeleted())
		assert.False(t, auth.CanLogin())
	})

	t.Run("false quand inactif", func(t *testing.T) {
		auth := fixtures.NewTestAuth(fixtures.WithInactive())
		assert.False(t, auth.CanLogin())
	})

	t.Run("false quand suspendu", func(t *testing.T) {
		future := time.Now().UTC().Add(24 * time.Hour)
		auth := fixtures.NewTestAuth(fixtures.WithSuspended(future))
		assert.False(t, auth.CanLogin())
	})

	t.Run("true quand suspension expirée", func(t *testing.T) {
		past := time.Now().UTC().Add(-24 * time.Hour)
		auth := fixtures.NewTestAuth(fixtures.WithSuspended(past))
		assert.True(t, auth.CanLogin())
	})
}

// --- Suspend / UnSuspend ---

func TestAuth_Suspend(t *testing.T) {
	auth := fixtures.NewTestAuth()
	before := time.Now().UTC()
	endDate := time.Now().UTC().Add(48 * time.Hour)

	auth.Suspend(endDate)

	assert.True(t, auth.IsSuspended)
	require.NotNil(t, auth.SuspensionEndDate)
	assert.Equal(t, endDate, *auth.SuspensionEndDate)
	assert.True(t, auth.UpdatedAt.After(before) || auth.UpdatedAt.Equal(before))
}

func TestAuth_UnSuspend(t *testing.T) {
	future := time.Now().UTC().Add(24 * time.Hour)
	auth := fixtures.NewTestAuth(fixtures.WithSuspended(future))
	before := time.Now().UTC()

	auth.UnSuspend()

	assert.False(t, auth.IsSuspended)
	assert.Nil(t, auth.SuspensionEndDate)
	assert.True(t, auth.UpdatedAt.After(before) || auth.UpdatedAt.Equal(before))
}

// --- Activate / Deactivate ---

func TestAuth_Activate(t *testing.T) {
	auth := fixtures.NewTestAuth(fixtures.WithInactive())
	before := time.Now().UTC()

	auth.Activate()

	assert.True(t, auth.IsActive)
	assert.True(t, auth.UpdatedAt.After(before) || auth.UpdatedAt.Equal(before))
}

func TestAuth_Deactivate(t *testing.T) {
	auth := fixtures.NewTestAuth()
	before := time.Now().UTC()

	auth.Deactivate()

	assert.False(t, auth.IsActive)
	assert.True(t, auth.UpdatedAt.After(before) || auth.UpdatedAt.Equal(before))
}

// --- AnonymizeAndDelete ---

func TestAuth_AnonymizeAndDelete(t *testing.T) {
	auth := fixtures.NewTestAuth(
		fixtures.WithAuthID("auth-123"),
		fixtures.WithFirebaseID("firebase-abc"),
		fixtures.WithEmail("user@example.com"),
		fixtures.WithPhoneNumber("+22890000000"),
	)
	before := time.Now().UTC()

	auth.AnonymizeAndDelete()

	// FirebaseID anonymisé
	assert.Equal(t, "deleted_auth-123", auth.FirebaseID)

	// Email anonymisé
	require.NotNil(t, auth.Email)
	assert.Contains(t, *auth.Email, "deleted_user_auth-123@anonymized.local")

	// Téléphone supprimé
	assert.Nil(t, auth.PhoneNumber)

	// Compte désactivé
	assert.False(t, auth.IsActive)

	// DeletedAt défini
	require.NotNil(t, auth.DeletedAt)
	assert.True(t, auth.DeletedAt.After(before) || auth.DeletedAt.Equal(before))

	// UpdatedAt mis à jour
	assert.True(t, auth.UpdatedAt.After(before) || auth.UpdatedAt.Equal(before))

	// L'objet est considéré comme supprimé
	assert.True(t, auth.IsDeleted())
	assert.False(t, auth.CanLogin())
}

// --- Test de la struct UserPreview ---

func TestUserPreview(t *testing.T) {
	email := "test@example.com"
	phone := "+22890000000"
	photo := "https://example.com/photo.jpg"

	preview := &domain.UserPreview{
		AuthID:          "auth-123",
		UserID:          "user-456",
		Name:            "Doe",
		FirstName:       "John",
		Email:           &email,
		PhoneNumber:     &phone,
		ProfilePhotoURL: &photo,
	}

	assert.Equal(t, "auth-123", preview.AuthID)
	assert.Equal(t, "user-456", preview.UserID)
	assert.Equal(t, "Doe", preview.Name)
	assert.Equal(t, "John", preview.FirstName)
	assert.Equal(t, "test@example.com", *preview.Email)
	assert.Equal(t, "+22890000000", *preview.PhoneNumber)
	assert.Equal(t, "https://example.com/photo.jpg", *preview.ProfilePhotoURL)
}
