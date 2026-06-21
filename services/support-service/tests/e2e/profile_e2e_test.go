package e2e

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/support-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
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

func TestE2E_UpdateMyProfile(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	u := fixtures.NewTestSupportUser(fixtures.WithEmail("agent@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))
	uctx := withSupport(ctx, u.UserID, domain.RoleSupport)

	t.Run("succès : nom et prénom mis à jour", func(t *testing.T) {
		_, err := d.client.UpdateMyProfile(uctx, &supportpb.UpdateMyProfileRequest{
			FirstName: "Jean", LastName: "Dupont",
		})
		require.NoError(t, err)

		me, err := d.client.Me(uctx, &supportpb.MeRequest{})
		require.NoError(t, err)
		assert.Equal(t, "Jean", me.GetFirstName())
		assert.Equal(t, "Dupont", me.GetLastName())
	})

	t.Run("prénom vide → InvalidArgument", func(t *testing.T) {
		_, err := d.client.UpdateMyProfile(uctx, &supportpb.UpdateMyProfileRequest{
			FirstName: "", LastName: "Dupont",
		})
		assert.Equal(t, codes.InvalidArgument, grpcCode(t, err))
	})
}

func TestE2E_UpdateSupportAgent_AdminFlow(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	admin := fixtures.NewTestAdmin(fixtures.WithEmail("admin@tissimah.local"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, admin))
	adminCtx := withSupport(ctx, admin.UserID, domain.RoleAdmin)

	target := fixtures.NewTestSupportUser(fixtures.WithEmail("agent@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, target))

	t.Run("admin modifie email + rôle", func(t *testing.T) {
		_, err := d.client.UpdateSupportAgent(adminCtx, &supportpb.UpdateSupportAgentRequest{
			UserId: target.UserID, Email: "promoted@x.com", Role: domain.RoleAdmin,
		})
		require.NoError(t, err)

		listRes, err := d.client.ListSupportAgents(adminCtx, &supportpb.ListSupportAgentsRequest{Limit: 50})
		require.NoError(t, err)
		var found *supportpb.SupportAgent
		for _, a := range listRes.GetAgents() {
			if a.GetUserId() == target.UserID {
				found = a
			}
		}
		require.NotNil(t, found)
		assert.Equal(t, "promoted@x.com", found.GetEmail())
		assert.Equal(t, domain.RoleAdmin, found.GetRole())
	})

	t.Run("non-admin → PermissionDenied", func(t *testing.T) {
		supportCtx := withSupport(ctx, target.UserID, domain.RoleSupport)
		_, err := d.client.UpdateSupportAgent(supportCtx, &supportpb.UpdateSupportAgentRequest{
			UserId: target.UserID, Role: domain.RoleSupport,
		})
		assert.Equal(t, codes.PermissionDenied, grpcCode(t, err))
	})
}

func TestE2E_Me_Unauthenticated(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()

	_, err := d.client.Me(context.Background(), &supportpb.MeRequest{})
	require.Error(t, err)
}
