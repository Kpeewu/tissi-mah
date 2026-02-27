package domain_test

import (
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/user-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- EnableDriverAccount / DisableDriverAccount ---

func TestUser_EnableDriverAccount(t *testing.T) {
	user := fixtures.NewTestUser()
	assert.False(t, user.IsDriver)
	before := time.Now().UTC()

	user.EnableDriverAccount()

	assert.True(t, user.IsDriver)
	assert.True(t, user.UpdatedAt.After(before) || user.UpdatedAt.Equal(before))
}

func TestUser_DisableDriverAccount(t *testing.T) {
	user := fixtures.NewTestUser(fixtures.WithDriver())
	assert.True(t, user.IsDriver)
	before := time.Now().UTC()

	user.DisableDriverAccount()

	assert.False(t, user.IsDriver)
	assert.True(t, user.UpdatedAt.After(before) || user.UpdatedAt.Equal(before))
}

// --- SetTripPreferences ---

func TestUser_SetTripPreferences(t *testing.T) {
	t.Run("définit les préférences", func(t *testing.T) {
		user := fixtures.NewTestUser()
		before := time.Now().UTC()

		prefs := []domain.TripPreference{
			{Preference: "music", IsAllowed: true},
			{Preference: "smoking", IsAllowed: false},
		}

		user.SetTripPreferences(prefs)

		require.Len(t, user.TripPreferences, 2)
		assert.Equal(t, "music", user.TripPreferences[0].Preference)
		assert.True(t, user.TripPreferences[0].IsAllowed)
		assert.Equal(t, "smoking", user.TripPreferences[1].Preference)
		assert.False(t, user.TripPreferences[1].IsAllowed)
		assert.True(t, user.UpdatedAt.After(before) || user.UpdatedAt.Equal(before))
	})

	t.Run("remplace les préférences existantes", func(t *testing.T) {
		user := fixtures.NewTestUser(fixtures.WithTripPreferences([]domain.TripPreference{
			{Preference: "old", IsAllowed: true},
		}))

		newPrefs := []domain.TripPreference{
			{Preference: "new", IsAllowed: false},
		}

		user.SetTripPreferences(newPrefs)

		require.Len(t, user.TripPreferences, 1)
		assert.Equal(t, "new", user.TripPreferences[0].Preference)
	})

	t.Run("accepte une slice vide", func(t *testing.T) {
		user := fixtures.NewTestUser(fixtures.WithTripPreferences([]domain.TripPreference{
			{Preference: "old", IsAllowed: true},
		}))

		user.SetTripPreferences([]domain.TripPreference{})

		assert.Empty(t, user.TripPreferences)
	})
}

// --- IsDeleted ---

func TestUser_IsDeleted(t *testing.T) {
	t.Run("false quand DeletedAt est nil", func(t *testing.T) {
		user := fixtures.NewTestUser()
		assert.False(t, user.IsDeleted())
	})

	t.Run("true quand DeletedAt est défini", func(t *testing.T) {
		user := fixtures.NewTestUser(fixtures.WithDeleted())
		assert.True(t, user.IsDeleted())
	})
}

// --- Valeurs par défaut NewTestUser ---

func TestNewTestUser_Defaults(t *testing.T) {
	user := fixtures.NewTestUser()

	assert.NotEmpty(t, user.UserID)
	assert.NotEmpty(t, user.AuthID)
	assert.NotEmpty(t, user.FirebaseID)
	assert.Equal(t, "Doe", user.Name)
	assert.Equal(t, "John", user.FirstName)
	assert.True(t, user.IsPassenger)
	assert.False(t, user.IsDriver)
	assert.False(t, user.HasProfileImage)
	assert.Nil(t, user.DeletedAt)
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}
