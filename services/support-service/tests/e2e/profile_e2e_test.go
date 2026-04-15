package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/support-service/fixtures"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestE2E_ChangeMyPassword(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	u := fixtures.NewTestSupportUser(fixtures.WithMustChangePassword(true))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))
	uctx := withSupport(ctx, u.UserID, "support")

	t.Run("succès + mustChange effacé", func(t *testing.T) {
		_, err := d.client.ChangeMyPassword(uctx, &supportpb.ChangeMyPasswordRequest{
			CurrentPassword: fixtures.DefaultPasswordPlain,
			NewPassword:     "NewPassw0rd!Test",
		})
		require.NoError(t, err)

		me, err := d.client.Me(uctx, &supportpb.MeRequest{})
		require.NoError(t, err)
		assert.False(t, me.GetMustChangePassword())
	})

	t.Run("ancien mot de passe incorrect → Unauthenticated", func(t *testing.T) {
		_, err := d.client.ChangeMyPassword(uctx, &supportpb.ChangeMyPasswordRequest{
			CurrentPassword: "bad",
			NewPassword:     "AnotherPassw0rd!",
		})
		assert.Equal(t, codes.Unauthenticated, grpcCode(t, err))
	})

	t.Run("nouveau mot de passe faible → InvalidArgument", func(t *testing.T) {
		_, err := d.client.ChangeMyPassword(uctx, &supportpb.ChangeMyPasswordRequest{
			CurrentPassword: "NewPassw0rd!Test",
			NewPassword:     "short",
		})
		assert.Equal(t, codes.InvalidArgument, grpcCode(t, err))
	})
}

func TestE2E_ChangeMyEmail(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	u := fixtures.NewTestSupportUser(fixtures.WithEmail("old@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))
	uctx := withSupport(ctx, u.UserID, "support")

	t.Run("succès", func(t *testing.T) {
		_, err := d.client.ChangeMyEmail(uctx, &supportpb.ChangeMyEmailRequest{
			NewEmail:        "newmail@x.com",
			CurrentPassword: fixtures.DefaultPasswordPlain,
		})
		require.NoError(t, err)

		me, err := d.client.Me(uctx, &supportpb.MeRequest{})
		require.NoError(t, err)
		assert.Equal(t, "newmail@x.com", me.GetEmail())
	})

	t.Run("cooldown 6 mois → FailedPrecondition", func(t *testing.T) {
		// Le premier change ci-dessus a posé email_changed_at = NOW → tentative immédiate bloquée.
		_, err := d.client.ChangeMyEmail(uctx, &supportpb.ChangeMyEmailRequest{
			NewEmail:        "yetanother@x.com",
			CurrentPassword: fixtures.DefaultPasswordPlain,
		})
		assert.Equal(t, codes.FailedPrecondition, grpcCode(t, err))
	})

	t.Run("email conflit", func(t *testing.T) {
		// Setup : un autre user avec email cible.
		other := fixtures.NewTestSupportUser(fixtures.WithEmail("taken@x.com"))
		require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, other))

		// User courant : reset email_changed_at pour contourner le cooldown.
		past := time.Now().Add(-7 * 30 * 24 * time.Hour)
		_, err := testPool.Exec(ctx,
			"UPDATE support_users SET email_changed_at = $1 WHERE user_id = $2",
			past, u.UserID,
		)
		require.NoError(t, err)

		_, err = d.client.ChangeMyEmail(uctx, &supportpb.ChangeMyEmailRequest{
			NewEmail:        "taken@x.com",
			CurrentPassword: fixtures.DefaultPasswordPlain,
		})
		assert.Equal(t, codes.AlreadyExists, grpcCode(t, err))
	})
}

func TestE2E_Me_Unauthenticated(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()

	_, err := d.client.Me(context.Background(), &supportpb.MeRequest{})
	require.Error(t, err)
}
